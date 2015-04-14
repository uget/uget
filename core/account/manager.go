package account

import (
	"encoding/json"
	log "github.com/cihub/seelog"
	"github.com/howeyc/fsnotify"
	"github.com/uget/uget/core"
	"github.com/uget/uget/utils"
	"io/ioutil"
	"os"
	path "path/filepath"
	"reflect"
	"sync"
)

func defaultFile() string {
	return path.Join(utils.ConfigPath(), "accounts.json")
}

type Account struct {
	// Selected bool        `json:"selected,omitempty"`
	Provider string      `json:"provider"`
	Data     interface{} `json:"data"`
}
type accstore map[string]Account
type root map[string]accstore

func (a *Account) UnmarshalJSON(bs []byte) error {
	var j struct {
		Provider string
		// Selected bool
		Data *json.RawMessage
	}
	if err := json.Unmarshal(bs, &j); err != nil {
		return err
	}
	data := core.GetProvider(j.Provider).(core.PersistentProvider).NewTemplate()
	json.Unmarshal(*j.Data, data)
	a.Provider = j.Provider
	// a.Selected = j.Selected
	a.Data = data
	return nil
}

type real_manager struct {
	file  string
	root  root
	queue chan *asyncJob
}

type Manager struct {
	*real_manager
	Provider core.PersistentProvider
}

type asyncJob struct {
	work func()
	done chan bool
}

var mtx = sync.Mutex{}
var managers = map[string]*real_manager{}

func ManagerFor(file string, p core.PersistentProvider) *Manager {
	if file == "" {
		file = defaultFile()
	}
	return &Manager{managerFor(file), p}
}

func managerFor(file string) *real_manager {
	mtx.Lock()
	defer mtx.Unlock()
	if managers[file] == nil {
		if err := os.MkdirAll(path.Dir(file), 0755); err != nil {
			log.Errorf("Could not create parent dirs of %s", file)
		}
		m := &real_manager{
			queue: make(chan *asyncJob),
			file:  file,
		}
		managers[file] = m
		go m.dispatch()
	}
	return managers[file]
}

// store of type *[]interface{} or panic!
func (m *Manager) Accounts(store interface{}) {
	<-m.job(func() {
		m.accounts(store)
	})
}

func (m *Manager) accounts(store interface{}) {
	t := reflect.Indirect(reflect.ValueOf(store))
	for _, v := range m.root[m.Provider.Name()] {
		t.Set(reflect.Append(t, reflect.Indirect(reflect.ValueOf(v.Data))))
	}
}

func (m *Manager) AddAccount(id string, store interface{}) {
	<-m.job(func() {
		m.addAccount(id, store)
	})
}

func (m *Manager) addAccount(id string, store interface{}) {
	m.p()[id] = Account{Provider: m.Provider.Name(), Data: store}
}

func (m *Manager) p() accstore {
	if _, ok := m.root[m.Provider.Name()]; !ok {
		m.root[m.Provider.Name()] = accstore{}
	}
	return m.root[m.Provider.Name()]
}

func (m *real_manager) save() error {
	b, err := json.MarshalIndent(m.root, "", "  ")
	if err != nil {
		return err
	}
	return ioutil.WriteFile(m.file, b, 0600)
}

func (m *real_manager) reload() {
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

func (m *Manager) job(f func()) <-chan bool {
	job := &asyncJob{
		work: f,
		done: make(chan bool, 1),
	}
	m.queue <- job
	return job.done
}

func (m *real_manager) dispatch() {
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
			if err := m.save(); err != nil {
				log.Errorf("Error saving file %s: %s", m.file, err)
			}
			// this is after m.save() because of race conditions that occur if main thread exits.
			// TODO: fix the race condition and move this up.
			job.done <- true
		}
	}
}
