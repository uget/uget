package core

import (
	"encoding/json"
	"io/ioutil"
	"os"
	path "path/filepath"
	"reflect"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/howeyc/fsnotify"
	"github.com/uget/uget/utils"
)

func defaultFile() string {
	return path.Join(utils.ConfigPath(), "accounts.json")
}

type accinfo struct {
	Selected bool    `json:"selected,omitempty"`
	Provider string  `json:"provider"`
	Data     Account `json:"data"`
}
type accstore map[string]*accinfo
type root map[string]accstore

func (a *accinfo) UnmarshalJSON(bs []byte) error {
	var j struct {
		Provider string
		Selected bool
		Data     *json.RawMessage
	}
	if err := json.Unmarshal(bs, &j); err != nil {
		return err
	}
	account := globalProviders.GetProvider(j.Provider).(Accountant).NewTemplate()
	json.Unmarshal(*j.Data, account)
	a.Provider = j.Provider
	a.Selected = j.Selected
	a.Data = account
	return nil
}

type internalAccMgr struct {
	jobber
	file string
	root root
}

// AccountManager manages provider accounts and keeps the accounts file and local memory in sync
type AccountManager struct {
	*internalAccMgr
	Provider Accountant
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
			logrus.Errorf("core.managerFor: could not create parent dirs of %s", file)
		}
		m := &internalAccMgr{jobber{make(chan *asyncJob)}, file, nil}
		managers[file] = m
		go m.dispatch()
	}
	return managers[file]
}

func (m *AccountManager) Accounts() []Account {
	var accs []Account
	<-m.job(func() {
		accs = m.accounts()
	})
	return accs
}

func (m *AccountManager) accounts() []Account {
	accMap := m.root[m.Provider.Name()]
	accounts := make([]Account, 0, len(accMap))
	for _, v := range accMap {
		acc := m.Provider.NewTemplate()
		icopy(acc, v.Data)
		accounts = append(accounts, acc)
	}
	return accounts
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
	m.p()[acc.ID()] = &accinfo{Provider: m.Provider.Name(), Data: acc}
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
				logrus.Errorf("internalAccMgr#reload: create %s: %v", m.file, err)
			} else {
				defer f.Close()
				_, err = f.WriteString("{}")
				if err != nil {
					logrus.Errorf("internalAccMgr#reload: write %s: %v", m.file, err)
				}
			}
			return
		}
		logrus.Errorf("internalAccMgr#reload: open %s: %v", m.file, err)
		return
	}
	defer f.Close()
	bytes, err := ioutil.ReadAll(f)
	if err != nil {
		logrus.Errorf("internalAccMgr#reload: read %s: %v", m.file, err)
		return
	}
	json.Unmarshal(bytes, &m.root)
}

func (m *internalAccMgr) dispatch() {
	m.reload()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logrus.Errorf("internalAccMgr#dispatch: could not initialize file watcher")
	} else {
		if err = watcher.Watch(m.file); err != nil {
			logrus.Errorf("internalAccMgr#dispatch: cannot watch %s", m.file)
		}
	}
	for {
		select {
		case ev := <-watcher.Event:
			if ev.IsModify() {
				m.reload()
			}
		case err := <-watcher.Error:
			logrus.Errorf("internalAccMgr#reload: error watching %s: %v", m.file, err)
		case job := <-m.jobQueue:
			job.work()
			if err := m.save(); err != nil {
				logrus.Errorf("internalAccMgr#reload: error saving %v", m.file)
			} else {
				logrus.Debugf("internalAccMgr#reload: sucess saving %v", m.file)
			}
			// this is after m.save() because of race conditions that occur if main thread exits.
			// TODO: fix the race condition and move this up.
			job.done <- struct{}{}
		}
	}
}
