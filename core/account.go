package core

import (
	"encoding/json"
	"io/ioutil"
	"os"
	path "path/filepath"
	"reflect"
	"sync"

	log "github.com/Sirupsen/logrus"
	"github.com/howeyc/fsnotify"
	"github.com/uget/uget/utils"
)

// Account represents a persistent record on a provider (useful e.g. to access restricted files)
type Account interface {
	// Returns a unique identifier for this account.
	// This will often be the username or e-mail.
	ID() string
}

func defaultFile() string {
	return path.Join(utils.ConfigPath(), "accounts.json")
}

type account struct {
	Selected bool        `json:"selected,omitempty"`
	Provider string      `json:"provider"`
	Data     interface{} `json:"data"`
}
type accstore map[string]*account
type root map[string]accstore

func (a *account) UnmarshalJSON(bs []byte) error {
	var j struct {
		Provider string
		Selected bool
		Data     *json.RawMessage
	}
	if err := json.Unmarshal(bs, &j); err != nil {
		return err
	}
	data := GetProvider(j.Provider).(Accountant).NewTemplate()
	json.Unmarshal(*j.Data, data)
	a.Provider = j.Provider
	a.Selected = j.Selected
	a.Data = data
	return nil
}

type internalAccMgr struct {
	file  string
	root  root
	queue chan *asyncJob
}

// AccountManager manages provider accounts and keeps the accounts file and local memory in sync
type AccountManager struct {
	*internalAccMgr
	Provider Accountant
}

type asyncJob struct {
	work func()
	done chan bool
}

var mtx = sync.Mutex{}
var managers = map[string]*internalAccMgr{}

// AccountManagerFor returns an AccountManager for the given file and provider. File can be empty.
func AccountManagerFor(file string, p Accountant) *AccountManager {
	if file == "" {
		file = defaultFile()
	}
	return &AccountManager{managerFor(file), p}
}

func managerFor(file string) *internalAccMgr {
	mtx.Lock()
	defer mtx.Unlock()
	if managers[file] == nil {
		if err := os.MkdirAll(path.Dir(file), 0755); err != nil {
			log.Errorf("Could not create parent dirs of %s", file)
		}
		m := &internalAccMgr{
			queue: make(chan *asyncJob),
			file:  file,
		}
		managers[file] = m
		go m.dispatch()
	}
	return managers[file]
}

// Accounts requires parameter of type `*[]interface{}` or panics
func (m *AccountManager) Accounts(store interface{}) {
	<-m.job(func() {
		m.accounts(store)
	})
}

func (m *AccountManager) accounts(store interface{}) {
	arr := reflect.Indirect(reflect.ValueOf(store))
	if arr.Kind() != reflect.Slice {
		panic("Must provide a slice")
	}
	isStrSlice := arr.Type().String() == "[]string"
	for id, v := range m.root[m.Provider.Name()] {
		var data interface{}
		if isStrSlice {
			data = id
		} else {
			data = v.Data
		}
		arr.Set(reflect.Append(arr, reflect.Indirect(reflect.ValueOf(data))))
	}
}

// SelectAccount sets a local account as selected
func (m *AccountManager) SelectAccount(id string) bool {
	var found bool
	<-m.job(func() {
		found = m.selectAccount(id)
	})
	return found
}

func (m *AccountManager) selectAccount(id string) bool {
	found := false
	for k, v := range m.p() {
		v.Selected = false
		if id == k {
			v.Selected = true
			found = true
		}
	}
	return found
}

// SelectedAccount returns the selected account (or the first if no selected exists)
// The returned bool indicates whether there was a selected account.
func (m *AccountManager) SelectedAccount() (Account, bool) {
	store := m.Provider.NewTemplate()
	var found, selected bool
	<-m.job(func() {
		found, selected = m.selectedAccount(store)
	})
	if !found {
		return nil, false
	}
	return store, selected
}

func (m *AccountManager) selectedAccount(store Account) (bool, bool) {
	none := true
	for _, v := range m.p() {
		if v.Selected {
			icopy(store, v.Data)
			return true, true
		} else if none {
			icopy(store, v.Data)
			none = false
		}
	}
	return !none, false
}

// AddAccount adds a record to the accounts file
func (m *AccountManager) AddAccount(account Account) {
	<-m.job(func() {
		m.addAccount(account)
	})
}

func (m *AccountManager) addAccount(acc Account) {
	m.p()[acc.ID()] = &account{Provider: m.Provider.Name(), Data: acc}
}

func (m *AccountManager) p() accstore {
	if _, ok := m.root[m.Provider.Name()]; !ok {
		m.root[m.Provider.Name()] = accstore{}
	}
	return m.root[m.Provider.Name()]
}

func icopy(dst, src interface{}) {
	if reflect.TypeOf(dst) != reflect.TypeOf(src) {
		panic("Unequal types")
	}
	tv := reflect.Indirect(reflect.ValueOf(src))
	av := reflect.Indirect(reflect.ValueOf(dst))
	for i := 0; i < tv.NumField(); i++ {
		av.Field(i).Set(tv.Field(i))
	}
}

func (m *AccountManager) job(f func()) <-chan bool {
	job := &asyncJob{
		work: f,
		done: make(chan bool, 1),
	}
	m.queue <- job
	return job.done
}

func (m *internalAccMgr) save() error {
	b, err := json.MarshalIndent(m.root, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(m.file, b, 0600)
}

func (m *internalAccMgr) reload() {
	m.root = root{}
	f, err := os.Open(m.file)
	if err != nil {
		if os.IsNotExist(err) {
			f, err = os.Create(m.file)
			if err != nil {
				log.Errorf("Could not create file %s: %v", m.file, err)
			} else {
				defer f.Close()
				_, err = f.WriteString("{}")
				if err != nil {
					log.Errorf("Could not write to file %s: %v", m.file, err)
				}
			}
			return
		}
		log.Errorf("Could not open file %s: %v", m.file, err)
		return
	}
	defer f.Close()
	bytes, err := ioutil.ReadAll(f)
	if err != nil {
		log.Errorf("Could not read file %s: %v", m.file, err)
		return
	}
	json.Unmarshal(bytes, &m.root)
}

func (m *internalAccMgr) dispatch() {
	m.reload()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Errorf("Could not initialize file watcher")
	} else {
		if err = watcher.Watch(m.file); err != nil {
			log.Errorf("Cannot watch %s", m.file)
		}
	}
	for {
		select {
		case ev := <-watcher.Event:
			if ev.IsModify() {
				m.reload()
			}
		case err := <-watcher.Error:
			log.Errorf("Error watching %s: %v", m.file, err)
		case job := <-m.queue:
			job.work()
			l := log.WithField("file", m.file)
			if err := m.save(); err != nil {
				l.WithField("err", err).Error("save ERROR!")
			} else {
				l.Debug("save SUCCESS!")
			}
			// this is after m.save() because of race conditions that occur if main thread exits.
			// TODO: fix the race condition and move this up.
			job.done <- true
		}
	}
}
