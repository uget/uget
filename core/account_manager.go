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

func defaultFile() string {
	return path.Join(utils.ConfigPath(), "accounts.json")
}

type Account struct {
	Selected bool        `json:"selected,omitempty"`
	Provider string      `json:"provider"`
	Data     interface{} `json:"data"`
}
type accstore map[string]*Account
type root map[string]accstore

func (a *Account) UnmarshalJSON(bs []byte) error {
	var j struct {
		Provider string
		Selected bool
		Data     *json.RawMessage
	}
	if err := json.Unmarshal(bs, &j); err != nil {
		return err
	}
	data := GetProvider(j.Provider).(PersistentProvider).NewTemplate()
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

type AccountManager struct {
	*internalAccMgr
	Provider PersistentProvider
}

type asyncJob struct {
	work func()
	done chan bool
}

var mtx = sync.Mutex{}
var managers = map[string]*internalAccMgr{}

func AccountManagerFor(file string, p PersistentProvider) *AccountManager {
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

// store of type *[]interface{} or panic!
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

// Get the selected account and copy its fields to `store`
// Returns 2 bools:
// The first indicates whether an account was found at all.
// The second indicates whether there's a selected account.
func (m *AccountManager) SelectedAccount(store interface{}) (bool, bool) {
	var found, selected bool
	<-m.job(func() {
		found, selected = m.selectedAccount(store)
	})
	return found, selected
}

func (m *AccountManager) selectedAccount(store interface{}) (bool, bool) {
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

func (m *AccountManager) AddAccount(id string, store interface{}) {
	<-m.job(func() {
		m.addAccount(id, store)
	})
}

func (m *AccountManager) addAccount(id string, store interface{}) {
	m.p()[id] = &Account{Provider: m.Provider.Name(), Data: store}
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
		} else {
			log.Errorf("Could not open file %s: %v", m.file, err)
			return
		}
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
