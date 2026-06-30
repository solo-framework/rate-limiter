package tests

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"ratelimiter/internal/config"
	"ratelimiter/internal/errors"
	"ratelimiter/service"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var groups = config.NewGroupList()

func TestMain(m *testing.M) {
	// setup
	err := groups.Add("default", 5, 10)
	if err != nil {
		panic(err)
	}

	code := m.Run()

	// teardown

	os.Exit(code)
}

func Test_time_provider(t *testing.T) {
	tp := service.NewTimeProvider()
	assert.NotNil(t, tp)
}

func Test_Manager_No_Group(t *testing.T) {
	cfg := getConfig()

	groups := config.NewGroupList()
	manager, err := service.NewManager(*groups, cfg.TTL, cfg.CleanupInterval, getTimeProvider())

	assert.Nil(t, manager)
	require.Error(t, err)
	require.ErrorIs(t, err, service.ErrNoGroups)
}

func Test_Manager_New(t *testing.T) {
	cfg := getConfig()
	manager, err := service.NewManager(*groups, cfg.TTL, cfg.CleanupInterval, getTimeProvider())

	assert.NotNil(t, manager)
	require.NoError(t, err)
}

func Test_Manager_New_wrong_ttl(t *testing.T) {
	cfg := getConfig()
	cfg.TTL = -1

	manager, err := service.NewManager(*groups, cfg.TTL, cfg.CleanupInterval, getTimeProvider())

	assert.Nil(t, manager)
	require.Error(t, err)
}

func Test_Manager_New_wrong_cleanupInterval(t *testing.T) {
	cfg := getConfig()
	cfg.CleanupInterval = -1

	manager, err := service.NewManager(*groups, cfg.TTL, cfg.CleanupInterval, getTimeProvider())

	assert.Nil(t, manager)
	require.Error(t, err)
}

func Test_Manager_err_empty_groups(t *testing.T) {
	cfg := getConfig()
	groups := config.NewGroupList()
	manager, err := service.NewManager(*groups, cfg.TTL, cfg.CleanupInterval, getTimeProvider())

	assert.Nil(t, manager)
	require.Error(t, err)
	require.ErrorIs(t, err, service.ErrNoGroups)
}

func Test_Manager_err_group_not_found(t *testing.T) {
	cfg := getConfig()
	manager, err := service.NewManager(*groups, cfg.TTL, cfg.CleanupInterval, getTimeProvider())

	require.NoError(t, err)
	require.NotNil(t, manager)

	tl, err := manager.GetLimiter("undefined-group", "user_id")

	require.Error(t, err)
	require.Nil(t, tl)
	assert.ErrorIs(t, err, service.ErrGroupNotFound)
}

func Test_Manager_get_Allow_ok(t *testing.T) {
	cfg := getConfig()
	groups = config.NewGroupList()
	err := groups.Add("lala", 5, 10)
	require.NoError(t, err)

	manager, err := service.NewManager(*groups, cfg.TTL, cfg.CleanupInterval, newFixedTimeProvider(100))

	require.NoError(t, err)
	require.NotNil(t, manager)

	// try to get limiter for a non-existing user
	tl, err := manager.GetLimiter("lala", "user_id")
	require.Error(t, err)
	require.ErrorIs(t, err, service.ErrLimiterNotFound)
	require.Nil(t, tl)

	// call Allow
	res, err := manager.Allow("lala", "user_id")

	require.NoError(t, err)
	require.True(t, res)

	// check limiter was created after Allow call
	tl, err = manager.GetLimiter("lala", "user_id")
	require.NoError(t, err)
	require.NotNil(t, tl)

	// and lastUse field is set
	assert.Equal(t, int64(100), tl.GetLastUse())
	assert.NotNil(t, tl.Get())

	// query again for the same group and user
	tl, err = manager.GetLimiter("lala", "user_id")

	require.NoError(t, err)
	require.NotNil(t, tl)

	// and ensure lastUse is unchanged after GetLimiter
	assert.Equal(t, int64(100), tl.GetLastUse())
	assert.NotNil(t, tl.Get())

	// assert.LessOrEqual(t, tl.GetLastUse(), time.Now().Unix())
	// assert.NotNil(t, tl.Get())
}

func Test_Allow_not_existing_group(t *testing.T) {
	cfg := getConfig()
	groups = config.NewGroupList()
	err := groups.Add("lala", 5, 10)
	require.NoError(t, err)

	manager, err := service.NewManager(*groups, cfg.TTL, cfg.CleanupInterval, getTimeProvider())
	require.NoError(t, err)
	require.NotNil(t, manager)

	res, err := manager.Allow("not defined group", "user_id")

	require.Error(t, err)
	require.ErrorIs(t, err, service.ErrGroupNotFound)
	require.False(t, res)
}

func Test_Manager_GetIngo(t *testing.T) {
	cfg := getConfig()
	groups = config.NewGroupList()

	err := groups.Add("gr1", 5, 10)
	require.NoError(t, err)
	err = groups.Add("gr2", 5, 10)
	require.NoError(t, err)

	manager, err := service.NewManager(*groups, cfg.TTL, cfg.CleanupInterval, getTimeProvider())
	require.NoError(t, err)
	require.NotNil(t, manager)

	_, err = manager.Allow("gr1", "user-1")
	require.NoError(t, err)
	_, err = manager.Allow("gr2", "user-2")
	require.NoError(t, err)

	type data struct {
		Name  string
		Count int
		Rate  float64
		Burst int
	}

	r, _ := json.Marshal(manager.GetInfo())
	var info []data
	_ = json.Unmarshal(r, &info)

	assert.Equal(t, 2, len(info))
	assert.Equal(t, 1, info[0].Count)
	assert.Equal(t, 1, info[1].Count)
}

func Test_Manager_AddGroup_via_Method(t *testing.T) {
	cfg := getConfig()
	groups = config.NewGroupList()
	err := groups.Add("gr1", 5, 10)
	require.NoError(t, err)

	manager, err := service.NewManager(*groups, cfg.TTL, cfg.CleanupInterval, getTimeProvider())
	require.NoError(t, err)
	require.NotNil(t, manager)

	t.Run("new group", func(t *testing.T) {
		err = manager.AddGroup("new_group", 10.0, 100)
		assert.NoError(t, err)

		stat := manager.GetStat()
		assert.Equal(t, 2, len(stat))

		_, ok := stat["new_group"]
		assert.True(t, ok)
	})

	t.Run("validate error name", func(t *testing.T) {
		err = manager.AddGroup("new#group", 10.0, 100)
		assert.ErrorIs(t, err, errors.ErrInvalidData)
		assert.ErrorContains(t, err, "must match pattern")
	})

	t.Run("validate error rate", func(t *testing.T) {
		err = manager.AddGroup("newgroup", -10.0, 100)
		assert.ErrorIs(t, err, errors.ErrInvalidData)
		assert.ErrorContains(t, err, "rate must be positive for")
	})

	t.Run("validate error burst", func(t *testing.T) {
		err = manager.AddGroup("newgroup", 10.0, -100)
		assert.ErrorIs(t, err, errors.ErrInvalidData)
		assert.ErrorContains(t, err, "burst must be positive for group")
	})

	t.Run("group exists error", func(t *testing.T) {
		err = manager.AddGroup("gr1", 10.0, 100)
		assert.ErrorIs(t, err, service.ErrGroupExists)

		stat := manager.GetStat()

		_, ok := stat["gr1"]
		assert.True(t, ok)
	})
}

func Test_Manager_AddGroup_via_Ctor(t *testing.T) {
	cfg := getConfig()
	groups = config.NewGroupList()

	err := groups.Add("gr1", 5, 10)
	require.NoError(t, err)
	err = groups.Add("gr2", 5, 10)
	require.NoError(t, err)

	// add groups via constructor
	manager, err := service.NewManager(*groups, cfg.TTL, cfg.CleanupInterval, getTimeProvider())
	require.NoError(t, err)
	require.NotNil(t, manager)

	_, err = manager.Allow("gr1", "user-1")
	require.NoError(t, err)
	_, err = manager.Allow("gr2", "user-2")
	require.NoError(t, err)

	type data struct {
		Name  string
		Count int
		Rate  float64
		Burst int
	}

	r, _ := json.Marshal(manager.GetInfo())
	var info []data
	_ = json.Unmarshal(r, &info)

	assert.Equal(t, 2, len(info))
	assert.Equal(t, 1, info[0].Count)
	assert.Equal(t, 1, info[1].Count)

	t.Run("Delete non existing group", func(t *testing.T) {
		manager.DeleteGroup("non-existing")

		r, _ := json.Marshal(manager.GetInfo())
		var info []data
		_ = json.Unmarshal(r, &info)

		assert.Equal(t, 2, len(info))
		assert.Equal(t, 1, info[0].Count)
		assert.Equal(t, 1, info[1].Count)
	})

	t.Run("Delete group", func(t *testing.T) {
		manager.DeleteGroup("gr1")
		r, _ := json.Marshal(manager.GetInfo())
		var info []data
		_ = json.Unmarshal(r, &info)

		assert.Equal(t, 1, len(info))
		assert.Equal(t, 1, info[0].Count)
		assert.Equal(t, "gr2", info[0].Name)
	})
}

func Test_Manager_UpdateGroup(t *testing.T) {
	cfg := getConfig()
	groups = config.NewGroupList()

	err := groups.Add("gr1", 5.0, 10)
	require.NoError(t, err)

	// add groups via constructor
	manager, err := service.NewManager(*groups, cfg.TTL, cfg.CleanupInterval, getTimeProvider())
	require.NoError(t, err)
	require.NotNil(t, manager)

	t.Run("OK", func(t *testing.T) {
		_, err = manager.Allow("gr1", "user-1")
		require.NoError(t, err)

		lim, _ := manager.GetLimiter("gr1", "user-1")

		assert.Equal(t, 5.0, float64(lim.Get().Limit()))
		assert.Equal(t, 10, lim.Get().Burst())

		err = manager.UpdateGroup("gr1", 1000.0, 1000)
		require.NoError(t, err)

		lim, _ = manager.GetLimiter("gr1", "user-1")
		assert.Equal(t, 1000.0, float64(lim.Get().Limit()))
		assert.Equal(t, 1000, lim.Get().Burst())
	})

	t.Run("non existing group", func(t *testing.T) {
		_, err = manager.Allow("gr1", "user-1")
		require.NoError(t, err)

		err = manager.UpdateGroup("gr_wrong_name", 1000.0, 1000)
		require.ErrorIs(t, err, service.ErrGroupNotFound)
	})
}

func Test_Allow_set_lastuse(t *testing.T) {
	cfg := getConfig()
	groups = config.NewGroupList()
	err := groups.Add("lala", 5, 10)
	require.NoError(t, err)

	manager, err := service.NewManager(*groups, cfg.TTL, cfg.CleanupInterval, newIncrementTimeProvider())

	require.NoError(t, err)
	require.NotNil(t, manager)

	// try to get limiter for a non-existing user
	tl, err := manager.GetLimiter("lala", "user_id")
	require.Error(t, err)
	require.ErrorIs(t, err, service.ErrLimiterNotFound)
	require.Nil(t, tl)

	// call Allow
	res, err := manager.Allow("lala", "user_id")

	require.NoError(t, err)
	require.True(t, res)

	// check lastUse changed after Allow call
	tl, err = manager.GetLimiter("lala", "user_id")
	require.NoError(t, err)
	require.NotNil(t, tl)
	assert.Equal(t, int64(1), tl.GetLastUse())

	// calling GetLimiter again must NOT change lastUse (only Allow func updates it)
	tl, err = manager.GetLimiter("lala", "user_id")
	require.NoError(t, err)
	require.NotNil(t, tl)
	assert.Equal(t, int64(1), tl.GetLastUse())
}

func Test_Cleanup_Concurrency(t *testing.T) {
	// just verify code does not crash due to race conditions

	if testing.Short() {
		t.Skipf("Test_Cleanup_Concurrency skipped in short mode")
	}

	defer func() {
		ok := true
		if r := recover(); r != nil {
			ok = false
			log.Println(r)
		}

		require.True(t, ok)
	}()

	cfg := getConfig()
	cfg.CleanupInterval = 1
	cfg.TTL = 1

	groupSourse := []string{"g1", "g2", "g3"}

	groups = config.NewGroupList()
	_ = groups.Add(groupSourse[0], 1, 10)
	_ = groups.Add(groupSourse[1], 1, 10)
	_ = groups.Add(groupSourse[2], 1, 10)

	manager, err := service.NewManager(*groups, cfg.TTL, cfg.CleanupInterval, getTimeProvider())
	require.NoError(t, err)

	stopCh := make(chan struct{})
	// defer func() { stopCh <- struct{}{} }()

	go func() {
		for {
			select {
			case <-stopCh:
				return

			default:
				log.Println(manager.GetStat())
				time.Sleep(500 * time.Millisecond)
			}
		}
	}()

	ctx, cancelFn := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelFn()

	// start clean
	go func() { manager.StartCleanup() }()

	// continuously generate limiters in each group
	go func() {
		c := 0
		for {
			c++
			select {
			case <-stopCh:
				return
			case <-ctx.Done():
				return

			default:
				_, err1 := manager.Allow("g1", fmt.Sprintf("userid_g1_%d", c))
				if err1 != nil {
					panic(err1)
				}
				_, err1 = manager.Allow("g2", fmt.Sprintf("userid_g2_%d", c))
				if err1 != nil {
					panic(err1)
				}
				_, err1 = manager.Allow("g3", fmt.Sprintf("userid_g3_%d", c))
				if err1 != nil {
					panic(err1)
				}
			}
		}
	}()

	time.Sleep(500 * time.Millisecond)

	// read
	go func() {
		c := 0
		for {
			c++
			select {
			case <-stopCh:
				return
			case <-ctx.Done():
				return

			default:
				_, _ = manager.GetLimiter("g1", fmt.Sprintf("userid_g1_%d", c))

				// if err1 != nil {
				// 	panic(err1)
				// }
				_, _ = manager.GetLimiter("g2", fmt.Sprintf("userid_g2_%d", c))
				// if err1 != nil {
				// 	panic(err1)
				// }
				_, _ = manager.GetLimiter("g3", fmt.Sprintf("userid_g3_%d", c))
				// if err1 != nil {
				// 	panic(err1)
				// }
			}
		}
	}()

	time.Sleep(2000 * time.Millisecond)
	manager.StopCleanup()
	stopCh <- struct{}{}

	stat := manager.GetStat()
	log.Printf("final: %+v", stat)
}

func Test_Cleanup_expired(t *testing.T) {
	if testing.Short() {
		t.Skipf("Benchmark_GetLimiter skipped in short mode")
	}

	cfg := getConfig()
	cfg.CleanupInterval = 1
	cfg.TTL = 10 * time.Millisecond // if limiter is not used during this time, it should expire

	groupSourse := []string{"g1", "g2", "g3"}

	groups = config.NewGroupList()
	_ = groups.Add(groupSourse[0], 1, 10)
	_ = groups.Add(groupSourse[1], 1, 10)
	_ = groups.Add(groupSourse[2], 1, 10)

	expiredTimeProvider := &expiredTimeProvider{
		now: 100, // assume current time is this value; all limiters start with it in lastUse
	}

	manager, err := service.NewManager(*groups, cfg.TTL, cfg.CleanupInterval, expiredTimeProvider)
	require.NoError(t, err)

	// generate limiters in each group
	go func() {
		for _, g := range groupSourse {
			for i := range 1000 {
				_, _ = manager.Allow(g, fmt.Sprintf("userid_%d", i))
				// if err1 != nil {
				// 	panic(err1)
				// }
			}
			// log.Printf("added 10_000 to %s\n", g)
			// time.Sleep(10 * time.Millisecond)
		}
	}()

	// wait until all limiters are created
	time.Sleep(100 * time.Millisecond)
	// move time one minute ahead; limiters should already be expired
	expiredTimeProvider.Advance(60)

	go manager.StartCleanup()
	// sleep for a while to ensure cleanup has started
	time.Sleep(1500 * time.Millisecond)

	manager.StopCleanup()

	stat := manager.GetStat()

	assert.Equal(t, 0, stat["g1"])
	assert.Equal(t, 0, stat["g2"])
	assert.Equal(t, 0, stat["g3"])
}

func Test_Manager_UpdateGroup_Concurrency(t *testing.T) {
	cfg := getConfig()

	groupSourse := []string{"g1", "g2", "g3"}

	groups = config.NewGroupList()
	_ = groups.Add(groupSourse[0], 1, 10)
	_ = groups.Add(groupSourse[1], 1, 10)
	_ = groups.Add(groupSourse[2], 1, 10)

	manager, err := service.NewManager(*groups, cfg.TTL, cfg.CleanupInterval, getTimeProvider())
	require.NoError(t, err)
	require.NotNil(t, manager)

	// stopCh := make(chan struct{})
	ctx, cancelFn := context.WithCancel(context.Background())

	wg := sync.WaitGroup{}

	// continuously generate limiters in each group
	go func() {
		for _, groupName := range groupSourse {
			wg.Add(1)
			go func(name string) {
				wg.Done()

				i := 0
				for {
					select {
					case <-ctx.Done():
						return
					default:
						_, err1 := manager.Allow(name, fmt.Sprintf("userid_%d", i))
						i++
						if err1 != nil {
							panic(err1)
						}
					}
				}
			}(groupName)
		}
	}()

	wg.Go(func() {
		time.Sleep(10 * time.Millisecond)
		err = manager.UpdateGroup("g2", 2000.0, 2000)
		assert.NoError(t, err)
	})

	time.Sleep(100 * time.Millisecond)
	cancelFn()

	wg.Wait()

	type data struct {
		Name  string
		Count int
		Rate  float64
		Burst int
	}

	r, _ := json.Marshal(manager.GetInfo())
	var info []data
	_ = json.Unmarshal(r, &info)

	for _, inf := range info {
		if inf.Name == "g2" {
			assert.Equal(t, 2000.0, inf.Rate)
			assert.Equal(t, 2000, inf.Burst)
		}
	}

	lim, err := manager.GetLimiter("g2", "userid_1")
	assert.NoError(t, err)

	assert.Equal(t, 2000.0, float64(lim.Get().Limit()))
	assert.Equal(t, 2000, lim.Get().Burst())
	// println(lim.Get().Limit())
}

func Test_Manager_SaveToFile_And_LoadFromFile(t *testing.T) {
	cfg := getConfig()

	// add groups
	sourceGroups := config.NewGroupList()
	err := sourceGroups.Add("api", 10, 20)
	require.NoError(t, err)

	err = sourceGroups.Add("auth", 3, 6)
	require.NoError(t, err)

	sourceManager, err := service.NewManager(*sourceGroups, cfg.TTL, cfg.CleanupInterval, getTimeProvider())
	require.NoError(t, err)

	// save groups
	tmpDir := t.TempDir()
	err = sourceManager.SaveGoupsToFile(tmpDir)
	require.NoError(t, err)

	// data file exists
	_, err = os.Stat(filepath.Join(tmpDir, "data.bin"))
	require.NoError(t, err)

	// load groups from config
	targetGroups := config.NewGroupList()
	err = targetGroups.Add("api", 1, 1)
	require.NoError(t, err)
	err = targetGroups.Add("legacy", 7, 8)
	require.NoError(t, err)

	targetManager, err := service.NewManager(*targetGroups, cfg.TTL, cfg.CleanupInterval, getTimeProvider())
	require.NoError(t, err)

	// load saved groups from file
	loaded, loadErr := targetManager.LoadGroupsFromFile(tmpDir)
	require.NoError(t, loadErr)
	// require.True(t, loaded)
	require.Equal(t, 2, loaded)

	type data struct {
		Name  string  `json:"name"`
		Count int     `json:"count"`
		Rate  float64 `json:"rate"`
		Burst int     `json:"burst"`
	}

	source := make([]data, 0)
	source = append(source, data{Name: "api", Count: 0, Rate: 10, Burst: 20})
	source = append(source, data{Name: "auth", Count: 0, Rate: 3, Burst: 6})
	source = append(source, data{Name: "legacy", Count: 0, Rate: 7, Burst: 8})

	// now in manager
	groupsData := targetManager.GetInfo()

	var target []data
	groupBytes, err := json.Marshal(groupsData)
	assert.NoError(t, err)
	err = json.Unmarshal(groupBytes, &target)
	assert.NoError(t, err)

	assert.ElementsMatch(t, source, target)
}

func Test_Manager_LoadFromFile_NotFound(t *testing.T) {
	cfg := getConfig()
	baseGroups := config.NewGroupList()
	err := baseGroups.Add("api", 1, 1)
	require.NoError(t, err)

	manager, err := service.NewManager(*baseGroups, cfg.TTL, cfg.CleanupInterval, getTimeProvider())
	require.NoError(t, err)

	loaded, loadErr := manager.LoadGroupsFromFile(t.TempDir())
	require.NoError(t, loadErr)
	// require.False(t, loaded)
	require.Equal(t, 0, loaded)
}

func Test_Manager_LoadFromFile_BadFile(t *testing.T) {
	cfg := getConfig()
	baseGroups := config.NewGroupList()
	err := baseGroups.Add("api", 1, 1)
	require.NoError(t, err)

	manager, err := service.NewManager(*baseGroups, cfg.TTL, cfg.CleanupInterval, getTimeProvider())
	require.NoError(t, err)

	dir := t.TempDir()
	randomData := []byte{0, 0, 1, 1}

	err = manager.SaveGoupsToFile(dir)
	assert.NoError(t, err)

	file, err := os.OpenFile(dir+"/data.bin", os.O_TRUNC|os.O_WRONLY, os.ModeAppend)
	defer func() { file.Close() }()

	assert.NoError(t, err)

	// damage file
	_, err = file.Write(randomData)
	assert.NoError(t, err)

	loaded, loadErr := manager.LoadGroupsFromFile(dir)
	require.Error(t, loadErr)
	// require.False(t, loaded)
	require.Equal(t, 0, loaded)
}

type expiredTimeProvider struct {
	now int64
	mu  sync.Mutex
}

// Now implements [service.ITimeProvider].
func (f *expiredTimeProvider) Now() int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.now
}

// func (f *expiredTimeProvider) Now() int64      { return f.now }
func (f *expiredTimeProvider) Advance(d int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.now += d
}
