package app

import (
	"encoding/json"
	"io/ioutil"
	"os"
	path "path/filepath"
	"reflect"
	"sync"

	"github.com/Sirupsen/logrus"
	"github.com/howeyc/fsnotify"
	"github.com/uget/uget/core"
	"github.com/uget/uget/utils"
)

func defaultFile() string {
	return path.Join(utils.ConfigPath(), "accounts.json")
}

type accinfo struct {
	Disabled bool         `json:"disabled,omitempty"`
	Provider string       `json:"provider"`
	Data     core.Account `json:"data"`
}
type accstore map[string]*accinfo
type root map[string]accstore

func (a *accinfo) UnmarshalJSON(bs []byte) error {
	var j struct {
		Provider string
		Disabled bool
		Data     *json.RawMessage
	}
	if err := json.Unmarshal(bs, &j); err != nil {
		return err
	}
	account := core.RegisteredProviders().GetProvider(j.Provider).(core.Accountant).NewTemplate()
	json.Unmarshal(*j.Data, account)
	a.Provider = j.Provider
	a.Disabled = j.Disabled
	a.Data = account
	return nil
}

type internalAccMgr struct {
	*utils.Jobber
	file string
	root root
}

// AccountManager manages provider accounts and keeps the accounts file and local memory in sync
type AccountManager struct {
	*internalAccMgr
	Provider core.Accountant
}

var mtx = sync.Mutex{}
var managers = map[string]*internalAccMgr{}

// AccountManagerFor returns an AccountManager for the given file and provider. File can be empty.
func AccountManagerFor(file string, p core.Accountant) *AccountManager {
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
			return nil
		}
		m := &internalAccMgr{utils.NewJobber(), file, nil}
		managers[file] = m
		go m.dispatch()
	}
	return managers[file]
}

func (m *AccountManager) Metadata() []*accinfo {
	var accs []*accinfo
	<-m.Job(func() {
		accs = m.metadata()
	})
	return accs
}

func (m *AccountManager) metadata() []*accinfo {
	accMap := m.root[m.Provider.Name()]
	accounts := make([]*accinfo, 0, len(accMap))
	for _, v := range accMap {
		accounts = append(accounts, v)
	}
	return accounts
}

func (m *AccountManager) Accounts() []core.Account {
	var accs []core.Account
	<-m.Job(func() {
		accs = m.accounts()
	})
	return accs
}

func (m *AccountManager) accounts() []core.Account {
	accMap := m.root[m.Provider.Name()]
	accounts := make([]core.Account, 0, len(accMap))
	for _, v := range accMap {
		if !v.Disabled {
			acc := m.Provider.NewTemplate()
			icopy(acc, v.Data)
			accounts = append(accounts, acc)
		}
	}
	return accounts
}

// DisableAccount disables an account from being used
func (m *AccountManager) DisableAccount(id string) bool {
	var found bool
	<-m.Job(func() {
		found = m.disableAccount(id)
	})
	return found
}

func (m *AccountManager) disableAccount(id string) bool {
	for k, v := range m.p() {
		if id == k {
			v.Disabled = true
			return true
		}
	}
	return false
}

// EnableAccount enables an account
func (m *AccountManager) EnableAccount(id string) bool {
	var found bool
	<-m.Job(func() {
		found = m.enableAccount(id)
	})
	return found
}

func (m *AccountManager) enableAccount(id string) bool {
	for k, v := range m.p() {
		if id == k {
			v.Disabled = false
			return true
		}
	}
	return false
}

// AddAccount adds a record to the accounts file
func (m *AccountManager) AddAccount(account core.Account) {
	<-m.Job(func() {
		m.addAccount(account)
	})
}

func (m *AccountManager) addAccount(acc core.Account) {
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
		case job := <-m.JobQueue:
			job.Work()
			if err := m.save(); err != nil {
				logrus.Errorf("internalAccMgr#reload: error saving %v", m.file)
			} else {
				logrus.Debugf("internalAccMgr#reload: sucess saving %v", m.file)
			}
			// this is after m.save() because of race conditions that occur if main thread exits.
			// TODO: fix the race condition and move this up.
			job.Done <- struct{}{}
		}
	}
}
