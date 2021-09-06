package resource

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
	"time"

	"plexobject.com/formicary/queen/types"

	"plexobject.com/formicary/internal/queue"
	common "plexobject.com/formicary/internal/types"
	"plexobject.com/formicary/queen/config"
)

const testIncomingTopic = "test-incoming-topic"

func Test_ShouldFindAntsForGivenMethodsAndTasks(t *testing.T) {
	// GIVEN resource manager is constructed
	conf := config.TestServerConfig()
	err := conf.Validate()
	require.NoError(t, err)
	client := queue.NewStubClient(&conf.CommonConfig)
	mgr := New(conf, client)
	err = mgr.Start(context.Background())
	require.NoError(t, err)

	// BEFORE registration, ants should not be available
	err = mgr.HasAntsForJobTags(
		[]common.TaskMethod{"DOCKER", "KUBERNETES"},
		[]string{"client-1", "aws"},
	)
	require.Error(t, err)

	// WHEN ants are registered with required methods and tags
	err = registerAnt(
		client,
		conf.GetRegistrationTopic(),
		[]common.TaskMethod{"DOCKER"},
		[]string{"client-1", "aws", "azure"},
		1)
	require.NoError(t, err)
	err = registerAnt(
		client,
		conf.GetRegistrationTopic(),
		[]common.TaskMethod{"KUBERNETES"},
		[]string{"client-2", "aws", "azure"},
		1)
	require.NoError(t, err)

	// THEN ants should be available
	if err := mgr.HasAntsForJobTags(
		[]common.TaskMethod{"DOCKER", "KUBERNETES"},
		[]string{"client-1", "aws"},
	); err != nil {
		t.Fatalf("expected availability %v", err)
	}
	err = mgr.Stop(context.Background())
	require.NoError(t, err)
}

func Test_ShouldNotFindAntsWithoutRequiredMethods(t *testing.T) {
	// GIVEN resource manager is constructed
	conf := config.TestServerConfig()
	err := conf.Validate()
	require.NoError(t, err)
	client := queue.NewStubClient(&conf.CommonConfig)

	mgr := New(conf, client)
	err = mgr.Start(context.Background())
	require.NoError(t, err)

	//
	// WHEN ants are registered with required methods and partially supported tags
	err = registerAnt(
		client,
		conf.GetRegistrationTopic(),
		[]common.TaskMethod{"DOCKER"},
		[]string{"client-1", "aws", "azure"},
		1)
	require.NoError(t, err)
	err = registerAnt(
		client,
		conf.GetRegistrationTopic(),
		[]common.TaskMethod{"KUBERNETES"},
		[]string{"client-2", "aws", "azure"},
		1)
	require.NoError(t, err)

	// THEN resource manager should not find ants if required method `SHELL` is missing
	err = mgr.HasAntsForJobTags(
		[]common.TaskMethod{"DOCKER", "KUBERNETES", "SHELL"},
		[]string{"client-1", "aws"},
	)
	require.Error(t, err)

	// Cleanup
	err = mgr.Stop(context.Background())
	require.NoError(t, err)
}

func Test_ShouldNotFindAntsWithoutRequiredTags(t *testing.T) {
	// GIVEN resource manager is constructed
	conf := config.TestServerConfig()
	err := conf.Validate()
	require.NoError(t, err)
	client := queue.NewStubClient(&conf.CommonConfig)
	mgr := New(conf, client)
	err = mgr.Start(context.Background())
	require.NoError(t, err)

	// WHEN ants are registered with required methods and partially supported tags
	//
	err = registerAnt(
		client,
		conf.GetRegistrationTopic(),
		[]common.TaskMethod{"DOCKER"},
		[]string{"client-1", "aws", "azure"},
		1)
	require.NoError(t, err)
	err = registerAnt(
		client,
		conf.GetRegistrationTopic(),
		[]common.TaskMethod{"KUBERNETES", "SHELL"},
		[]string{"client-2", "aws", "azure"},
		1)
	require.NoError(t, err)

	// THEN resource manager should not find ants if required tag `google` is missing
	err = mgr.HasAntsForJobTags(
		[]common.TaskMethod{"DOCKER", "KUBERNETES", "SHELL"},
		[]string{"client-1", "aws", "google"},
	)
	require.Error(t, err)

	err = mgr.Stop(context.Background())
	require.NoError(t, err)
}

func Test_ShouldReturnErrorWhenReleasingWithoutReservingFirst(t *testing.T) {
	// GIVEN resource manager is constructed
	conf := config.TestServerConfig()
	err := conf.Validate()
	require.NoError(t, err)
	client := queue.NewStubClient(&conf.CommonConfig)
	mgr := New(conf, client)
	err = mgr.Start(context.Background())
	require.NoError(t, err)

	//
	// WHEN releasing ants without reservation
	// THEN error should return
	err = mgr.Release(&common.AntReservation{})
	require.Error(t, err)

	// Cleanup
	err = mgr.Stop(context.Background())
	require.NoError(t, err)
}

func Test_ShouldReserveTasks(t *testing.T) {
	// GIVEN resource manager is constructed

	conf := config.TestServerConfig()
	err := conf.Validate()
	require.NoError(t, err)
	client := queue.NewStubClient(&conf.CommonConfig)
	mgr := New(conf, client)
	err = mgr.Start(context.Background())
	require.NoError(t, err)

	// WHEN Reserving without registration
	alloc, err := mgr.Reserve(1, "task", "DOCKER", []string{"client-1", "aws"})
	// THEN it should fail
	require.Error(t, err)

	// WHEN registering with method and tags
	err = registerAnt(
		client,
		conf.GetRegistrationTopic(),
		[]common.TaskMethod{"DOCKER"},
		[]string{"client-1", "aws", "azure"},
		1)
	require.NoError(t, err)

	allocs := make([]*common.AntReservation, 0)
	for i := 0; i < 10; i++ {
		alloc, err = mgr.Reserve(uint64(i+10), "my-task", "DOCKER", []string{"client-1", "aws"})
		require.NoError(t, err)
		allocs = append(allocs, alloc)
	}

	// WHEN allocating next
	err = mgr.HasAntsForJobTags(
		[]common.TaskMethod{"DOCKER"},
		[]string{"client-1", "aws"},
	)
	// THEN it should fail
	require.Error(t, err)

	// WHEN allocating next
	err = mgr.HasAntsForJobTags(
		[]common.TaskMethod{"DOCKER"},
		[]string{"client-1", "aws"},
	)
	// THEN it should fail
	require.Error(t, err)

	// releasing
	for i := 0; i < 10; i++ {
		err = mgr.Release(allocs[i])
		if err != nil {
			t.Fatalf("unexpected error %v, i %d, alloc %v ", err, i, allocs[i])
		}
		// WHEN checking ants after release
		err = mgr.HasAntsForJobTags(
			[]common.TaskMethod{"DOCKER"},
			[]string{"client-1", "aws"},
		)
		// THEN it should not fail
		require.NoError(t, err)
	}

	err = mgr.Stop(context.Background())
	require.NoError(t, err)
}

func Test_ShouldReserveJobs(t *testing.T) {
	// GIVEN resource manager is constructed
	conf := config.TestServerConfig()
	err := conf.Validate()
	require.NoError(t, err)
	client := queue.NewStubClient(&conf.CommonConfig)
	mgr := New(conf, client)

	err = mgr.Start(context.Background())
	require.NoError(t, err)

	// WHEN registering with method and tags
	err = registerAnt(
		client,
		conf.GetRegistrationTopic(),
		[]common.TaskMethod{"DOCKER"},
		[]string{"client-1", "aws", "azure"},
		1)
	require.NoError(t, err)

	allocs := make([]map[string]*common.AntReservation, 0)
	// Each job has two tasks so 10 * 2 but each capacity is matched against requests not tasks
	for i := 0; i < 10; i++ {
		job := newTestJobDefinition(fmt.Sprintf("job-%d", i))
		job.ID = fmt.Sprintf("job-%d", i)
		reservations, err := mgr.ReserveJobResources(uint64(i+1), job)
		require.NoError(t, err)
		allocs = append(allocs, reservations)
	}

	// WHEN allocating for nonexisting tags
	err = mgr.HasAntsForJobTags(
		[]common.TaskMethod{"DOCKER"},
		[]string{"client-1", "aws"},
	)
	// THEN it should fail
	require.Error(t, err)

	// reserving resources
	for i := 0; i < 10; i++ {
		job := newTestJobDefinition(fmt.Sprintf("job-%d", i))
		job.ID = fmt.Sprintf("job-%d", i)
		reservations, err := mgr.ReserveJobResources(uint64(i+1), job)
		require.NoError(t, err)
		allocs = append(allocs, reservations)
	}

	// WHEN allocating for nonexisting tags
	err = mgr.HasAntsForJobTags(
		[]common.TaskMethod{"DOCKER"},
		[]string{"client-1", "aws"},
	)
	// THEN it should fail
	require.Error(t, err)

	// releasing
	for i := 0; i < 10; i++ {
		err := mgr.ReleaseJobResources(uint64(i + 1))
		if err != nil {
			t.Fatalf("expected allocation %v", err)
		}
		// WHEN allocating after release
		err = mgr.HasAntsForJobTags(
			[]common.TaskMethod{"DOCKER"},
			[]string{"client-1", "aws"},
		)
		// THEN it should not fail
		require.NoError(t, err)
	}

	err = mgr.Stop(context.Background())
	require.NoError(t, err)
}

func Test_ShouldFailReservationWithoutMethod(t *testing.T) {
	// GIVEN resource manager is constructed
	conf := config.TestServerConfig()
	err := conf.Validate()
	require.NoError(t, err)
	client := queue.NewStubClient(&conf.CommonConfig)
	mgr := New(conf, client)
	err = mgr.Start(context.Background())
	require.NoError(t, err)

	// WHEN registering with method and tags
	err = registerAnt(
		client,
		conf.GetRegistrationTopic(),
		[]common.TaskMethod{"DOCKER"},
		[]string{"client-1", "aws", "azure"},
		1)
	require.NoError(t, err)

	// THEN reservation should fail because `KUBERNETES` method is not available
	_, err = mgr.Reserve(1, "task", "KUBERNETES", []string{"client-1", "aws"})
	require.Error(t, err)
}

func Test_ShouldFailReservationWithoutTag(t *testing.T) {
	// GIVEN resource manager is constructed

	testAntID = 0
	conf := config.TestServerConfig()
	err := conf.Validate()
	require.NoError(t, err)
	client := queue.NewStubClient(&conf.CommonConfig)

	mgr := New(conf, client)
	err = mgr.Start(context.Background())
	require.NoError(t, err)

	// WHEN registering with method and tags
	err = registerAnt(
		client,
		conf.GetRegistrationTopic(),
		[]common.TaskMethod{"DOCKER"},
		[]string{"client-1", "aws", "azure"},
		1)
	require.NoError(t, err)
	err = registerAnt(
		client,
		conf.GetRegistrationTopic(),
		[]common.TaskMethod{"KUBERNETES"},
		[]string{"client-2", "aws", "google"},
		1)
	require.NoError(t, err)

	// WHEN reservation by tags `DOCKER` and `client-2` is not available
	_, err = mgr.Reserve(1, "task", "DOCKER", []string{"client-2", "aws"})
	// THEN reservation should fail because `DOCKER` and `client-2` is not available
	require.Error(t, err)
	err = mgr.Stop(context.Background())
	require.NoError(t, err)
}

func Test_ShouldReapStaleAllocations(t *testing.T) {
	// GIVEN resource manager is constructed

	testAntID = 0
	conf := config.TestServerConfig()
	err := conf.Validate()
	require.NoError(t, err)
	client := queue.NewStubClient(&conf.CommonConfig)
	mgr := New(conf, client)
	err = mgr.Start(context.Background())
	require.NoError(t, err)

	// WHEN registering with method and tags
	err = registerAnt(
		client,
		conf.GetRegistrationTopic(),
		[]common.TaskMethod{"DOCKER"},
		[]string{"client-1", "aws", "azure"},
		1)
	require.NoError(t, err)
	err = registerAnt(
		client,
		conf.GetRegistrationTopic(),
		[]common.TaskMethod{"DOCKER", "SHELL"},
		[]string{"client-1", "aws", "azure", "local"},
		1)
	require.NoError(t, err)

	// THEN reservation should succeed up to max-capacity 10
	for i := 0; i < 10; i++ {
		alloc, err := mgr.Reserve(uint64(i), "my-task", "DOCKER", []string{"client-1", "aws"})
		require.NoError(t, err)
		require.Contains(t, alloc.AntID, "ant-id-") // ant-id-1 or ant-id-2
	}
	require.Equal(t, 2, len(mgr.state.allocationsByAnt))

	// AND reaping allocations should fail because AllocatedAt is recent
	require.Equal(t, 0, mgr.reapStaleAllocations(context.Background()) )
	conf.Jobs.AntReservationTimeout = 10 * time.Second

	// BUT after changing AllocatedAt to old date
	for _, allocs := range mgr.state.allocationsByAnt {
		for _, alloc := range allocs {
			alloc.AllocatedAt = time.Unix(0, 0)
		}
	}
	// THEN reaping allocations should succeed
	require.NotEqual(t, 0, mgr.reapStaleAllocations(context.Background()))

	// Cleanup
	err = mgr.Stop(context.Background())
	require.NoError(t, err)
}

func Test_ShouldReapStaleAnts(t *testing.T) {
	// GIVEN resource manager is constructed

	testAntID = 0
	conf := config.TestServerConfig()
	err := conf.Validate()
	require.NoError(t, err)
	client := queue.NewStubClient(&conf.CommonConfig)
	mgr := New(conf, client)
	err = mgr.Start(context.Background())
	require.NoError(t, err)

	// WHEN registering with method and tags
	err = registerAnt(
		client,
		conf.GetRegistrationTopic(),
		[]common.TaskMethod{"DOCKER"},
		[]string{"client-1", "aws", "azure"},
		1)
	require.NoError(t, err)
	err = registerAnt(
		client,
		conf.GetRegistrationTopic(),
		[]common.TaskMethod{"DOCKER", "SHELL"},
		[]string{"client-1", "aws", "azure", "local"},
		1)
	require.NoError(t, err)
	err = registerAnt(
		client,
		conf.GetRegistrationTopic(),
		[]common.TaskMethod{"KUBERNETES", "DOCKER"},
		[]string{"client-1", "aws", "google"},
		6)
	require.NoError(t, err)
	err = registerAnt(
		client,
		conf.GetRegistrationTopic(),
		[]common.TaskMethod{"KUBERNETES", "DOCKER"},
		[]string{"client-2", "aws", "google"},
		6)
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		// WHEN making reservation
		alloc, err := mgr.Reserve(uint64(i), "my-task", "DOCKER", []string{"client-1", "aws"})
		// THEN reservation should succeed
		require.NoError(t, err)
		require.Contains(t, alloc.AntID, "ant-id-") // ant-id-1 or ant-id-2
	}
	require.Equal(t, 4, len(mgr.state.antRegistrations))

	// AND reaping ants should not succeed
	count := mgr.reapStaleAnts(context.Background())
	require.Equal(t, 0, count)
	conf.Jobs.AntReservationTimeout = 10 * time.Second

	// BUT after changing received-at
	for _, reg := range mgr.state.antRegistrations {
		reg.ReceivedAt = time.Unix(0, 0)
	}

	// reaping should succeed
	count = mgr.reapStaleAnts(context.Background())
	require.NotEqual(t, 0, count)
	err = mgr.Stop(context.Background())
	require.NoError(t, err)
}

func Test_ShouldFindAntWithLeastLoad(t *testing.T) {
	// GIVEN resource manager is constructed

	testAntID = 0
	conf := config.TestServerConfig()
	err := conf.Validate()
	require.NoError(t, err)
	client := queue.NewStubClient(&conf.CommonConfig)
	mgr := New(conf, client)
	err = mgr.Start(context.Background())
	require.NoError(t, err)

	// WHEN registering with method and tags
	err = registerAnt(
		client,
		conf.GetRegistrationTopic(),
		[]common.TaskMethod{"DOCKER"},
		[]string{"client-1", "aws", "azure"},
		1)
	require.NoError(t, err)
	err = registerAnt(
		client,
		conf.GetRegistrationTopic(),
		[]common.TaskMethod{"DOCKER", "SHELL"},
		[]string{"client-1", "aws", "azure", "local"},
		1)
	require.NoError(t, err)
	err = registerAnt(
		client,
		conf.GetRegistrationTopic(),
		[]common.TaskMethod{"KUBERNETES", "DOCKER"},
		[]string{"client-1", "aws", "google"},
		6)
	require.NoError(t, err)
	err = registerAnt(
		client,
		conf.GetRegistrationTopic(),
		[]common.TaskMethod{"KUBERNETES", "DOCKER"},
		[]string{"client-2", "aws", "google"},
		6)
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		// WHEN making reservation
		alloc, err := mgr.Reserve(uint64(i), "my-task", "DOCKER", []string{"client-1", "aws"})
		// THEN reservation should succeed
		require.NoError(t, err)
		require.Contains(t, alloc.AntID, "ant-id-") // ant-id-1 or ant-id-2
	}

	for i := 0; i < 20; i++ {
		_, err = mgr.Reserve(uint64(i), "my-task", "DOCKER", []string{"client-1", "aws"})
		require.NoError(t, err)
	}

	// Cleanup
	err = mgr.Stop(context.Background())
	require.NoError(t, err)
}

func Test_ShouldIncrementLoadAfterAReservation(t *testing.T) {
	// GIVEN resource manager is constructed

	conf := config.TestServerConfig()
	err := conf.Validate()
	require.NoError(t, err)
	client := queue.NewStubClient(&conf.CommonConfig)
	mgr := New(conf, client)
	err = mgr.Start(context.Background())
	require.NoError(t, err)

	// WHEN registering with method and tags
	err = registerAnt(
		client,
		conf.GetRegistrationTopic(),
		[]common.TaskMethod{"DOCKER"},
		[]string{"client-1", "aws", "azure"},
		1)
	require.NoError(t, err)
	err = registerAnt(
		client,
		conf.GetRegistrationTopic(),
		[]common.TaskMethod{"KUBERNETES"},
		[]string{"client-2", "aws", "google"},
		3)
	require.NoError(t, err)
	err = registerAnt(
		client,
		conf.GetRegistrationTopic(),
		[]common.TaskMethod{"KUBERNETES"},
		[]string{"client-2", "aws", "google"},
		7)
	require.NoError(t, err)

	// THEN reservation add load
	alloc, err := mgr.Reserve(101, "task", "KUBERNETES", []string{"client-2", "aws"})
	require.NoError(t, err)
	require.Equal(t, 4, alloc.CurrentLoad)

	err = mgr.Stop(context.Background())
	require.NoError(t, err)
}

var testAntID int

func registerAnt(
	queueClient queue.Client,
	registrationTopic string,
	methods []common.TaskMethod,
	tags []string,
	load int) (err error) {
	testAntID++
	allocations := make(map[uint64]*common.AntAllocation)
	antID := fmt.Sprintf("ant-id-%d", testAntID)
	for i := 0; i < load; i++ {
		alloc := &common.AntAllocation{
			JobRequestID: uint64(i),
			TaskTypes:    map[string]common.RequestState{"task": common.EXECUTING},
			AntID:        antID,
			AllocatedAt:  time.Now(),
		}
		allocations[uint64(i)] = alloc
	}
	registration := common.AntRegistration{
		AntID:       antID,
		MaxCapacity: 10,
		Tags:        tags,
		Methods:     methods,
		Allocations: allocations,
	}

	registration.AntTopic = testIncomingTopic
	registration.CurrentLoad = load
	var b []byte
	if b, err = registration.Marshal(); err == nil {
		_, err = queueClient.Publish(
			context.Background(),
			registrationTopic,
			make(map[string]string),
			b,
			false)
	}
	return
}

func newTestJobDefinition(name string) *types.JobDefinition {
	job := types.NewJobDefinition(name)
	task1 := types.NewTaskDefinition("task1", common.Shell)
	task1.BeforeScript = []string{"t1_cmd1", "t1_cmd2", "t1_cmd3"}
	task1.Script = []string{"t1_cmd1", "t1_cmd2", "t1_cmd3"}
	task1.Method = common.Docker
	task1.OnExitCode["completed"] = "task2"

	task2 := types.NewTaskDefinition("task2", common.Shell)
	task2.BeforeScript = []string{"t2_cmd1", "t2_cmd2", "t2_cmd3"}
	task2.Script = []string{"t2_cmd1", "t2_cmd2", "t2_cmd3"}
	task2.Method = common.Docker
	task2.OnExitCode["completed"] = "task3"

	job.AddTask(task1)
	job.AddTask(task2)
	return job
}
