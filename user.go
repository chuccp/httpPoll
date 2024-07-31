package main

import (
	"sync"
	"time"
)

type User struct {
	Id             string
	UserId         string
	CreateTime     *time.Time
	LastUpdateTime *time.Time
	LastLeaveTime  *time.Time
	RemoteAddress  string
}

func (u *User) isExpired(expiredTime *time.Time) bool {
	if u.LastLeaveTime != nil {
		if u.LastLeaveTime.Before(*expiredTime) {
			return true
		}
	}
	return false
}

func NewUser(UserId string, RemoteAddress string) *User {
	return &User{UserId: UserId, RemoteAddress: RemoteAddress, Id: UserId + "_" + RemoteAddress}
}

type Users struct {
	users      map[string]*User
	CreateTime *time.Time
	UserId     string
	queue      *Queue
	lock       *sync.RWMutex
}

func NewUsers(UserId string, now *time.Time) *Users {
	return &Users{UserId: UserId, CreateTime: now, users: make(map[string]*User), queue: NewQueue(), lock: new(sync.RWMutex)}
}

func (us *Users) Wait(timer *time.Timer) (any, bool) {
	return us.queue.DequeueTimer(timer)
}

func (us *Users) Send(msg string) {
	us.queue.Offer(msg)
}

func (us *Users) UpdateLastLeaveTime(nu *User) {
	us.lock.RLock()
	defer us.lock.RUnlock()
	u, ok := us.users[nu.Id]
	if ok {
		now := time.Now()
		u.LastLeaveTime = &now
	}
}

func (us *Users) AddOrUpdateUser(nu *User, now *time.Time) {
	us.lock.Lock()
	defer us.lock.Unlock()
	u, ok := us.users[nu.Id]
	if ok {
		u.LastUpdateTime = now
		u.LastLeaveTime = nil
	} else {
		nu.CreateTime = now
		nu.LastUpdateTime = now
		nu.LastLeaveTime = nil
		us.users[nu.Id] = nu
	}
}
func (us *Users) DeleteUser(u *User) {
	us.lock.Lock()
	defer us.lock.Unlock()
	delete(us.users, u.Id)
}
func (us *Users) Num() int {
	us.lock.RLock()
	defer us.lock.RUnlock()
	return len(us.users)
}

func (us *Users) expiredCheck() {
	us.lock.Lock()
	defer us.lock.Unlock()
	t := time.Now().Add(-time.Second * 4)
	ids := make([]string, 0)
	for id, user := range us.users {
		if user.isExpired(&t) {
			ids = append(ids, id)
		}
	}
	for _, id := range ids {
		delete(us.users, id)
	}
}

type Store struct {
	userMap *sync.Map
	num     int
	lock    *sync.RWMutex
}

func NewStore() *Store {
	return &Store{userMap: new(sync.Map), num: 0, lock: new(sync.RWMutex)}
}

func (s *Store) AddUser(user *User) *Users {
	s.lock.Lock()
	defer s.lock.Unlock()
	now := time.Now()
	v, ok := s.userMap.Load(user.UserId)
	if ok {
		us := v.(*Users)
		us.AddOrUpdateUser(user, &now)
		return us
	} else {
		s.num++
		us := NewUsers(user.UserId, &now)
		us.AddOrUpdateUser(user, &now)
		s.userMap.Store(user.UserId, us)
		return us
	}
}

// 轮询检查离线用户
func (s *Store) loopCheck() {
	for {
		time.Sleep(time.Second)
		s.userMap.Range(func(userId, value any) bool {
			users := value.(*Users)
			users.expiredCheck()
			s.lock.Lock()
			if users.Num() == 0 {
				s.num--
				s.userMap.Delete(userId)
			}
			s.lock.Unlock()
			return true
		})
	}
}

func (s *Store) GetUser(userId string) *Users {
	s.lock.RLock()
	defer s.lock.RUnlock()
	v, ok := s.userMap.Load(userId)
	if ok {
		us := v.(*Users)
		return us
	}
	return nil
}

func (s *Store) DeleteUser(user *User) {
	s.lock.Lock()
	defer s.lock.Unlock()
	v, ok := s.userMap.Load(user.UserId)
	if ok {
		us := v.(*Users)
		us.DeleteUser(user)
		if us.Num() == 0 {
			s.num--
			s.userMap.Delete(user.UserId)
		}
	}
}
