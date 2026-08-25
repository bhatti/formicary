package resource

import (
	"context"
	"fmt"
	"github.com/oklog/ulid/v2"
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
	client, err := queue.NewClientManager().GetClient(context.Background(), &conf.Common)
	require.NoError(t, err)
	mgr := New(conf, client)
	err = mgr.Start(context.Background())
	require.NoError(t, err)

	// BEFORE registration, ants should not be available
	err = mgr.HasAntsForJobTags(
		[]common.TaskMethod{"DOCKER", "KUBERNETES"},
		[]string{"client-1", "aws"},
		"",
	)
	require.Error(t, err)

	// WHEN ants are registered with required methods and tags
	err = registerAnt(
		client,
		conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"DOCKER"},
		[]string{"client-1", "aws", "azure"},
		1)
	require.NoError(t, err)
	err = registerAnt(
		client,
		conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"KUBERNETES"},
		[]string{"client-2", "aws", "azure"},
		1)
	require.NoError(t, err)

	// THEN ants should be available
	if err := mgr.HasAntsForJobTags(
		[]common.TaskMethod{"DOCKER", "KUBERNETES"},
		[]string{"client-1", "aws"},
		"",
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
	client, err := queue.NewClientManager().GetClient(context.Background(), &conf.Common)
	require.NoError(t, err)

	mgr := New(conf, client)
	err = mgr.Start(context.Background())
	require.NoError(t, err)

	//
	// WHEN ants are registered with required methods and partially supported tags
	err = registerAnt(
		client,
		conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"DOCKER"},
		[]string{"client-1", "aws", "azure"},
		1)
	require.NoError(t, err)
	err = registerAnt(
		client,
		conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"KUBERNETES"},
		[]string{"client-2", "aws", "azure"},
		1)
	require.NoError(t, err)

	// THEN resource manager should not find ants if required method `SHELL` is missing
	err = mgr.HasAntsForJobTags(
		[]common.TaskMethod{"DOCKER", "KUBERNETES", "SHELL"},
		[]string{"client-1", "aws"},
		"",
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
	client, err := queue.NewClientManager().GetClient(context.Background(), &conf.Common)
	require.NoError(t, err)
	mgr := New(conf, client)
	err = mgr.Start(context.Background())
	require.NoError(t, err)

	// WHEN ants are registered with required methods and partially supported tags
	//
	err = registerAnt(
		client,
		conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"DOCKER"},
		[]string{"client-1", "aws", "azure"},
		1)
	require.NoError(t, err)
	err = registerAnt(
		client,
		conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"KUBERNETES", "SHELL"},
		[]string{"client-2", "aws", "azure"},
		1)
	require.NoError(t, err)

	// THEN resource manager should not find ants if required tag `google` is missing
	err = mgr.HasAntsForJobTags(
		[]common.TaskMethod{"DOCKER", "KUBERNETES", "SHELL"},
		[]string{"client-1", "aws", "google"},
		"",
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
	client, err := queue.NewClientManager().GetClient(context.Background(), &conf.Common)
	require.NoError(t, err)
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
	client, err := queue.NewClientManager().GetClient(context.Background(), &conf.Common)
	require.NoError(t, err)
	mgr := New(conf, client)
	err = mgr.Start(context.Background())
	require.NoError(t, err)

	// WHEN Reserving without registration
	alloc, err := mgr.Reserve(ulid.Make().String(), "task", "DOCKER", []string{"client-1", "aws"}, "")
	// THEN it should fail
	require.Error(t, err)

	// WHEN registering with method and tags
	err = registerAnt(
		client,
		conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"DOCKER"},
		[]string{"client-1", "aws", "azure"},
		1)
	require.NoError(t, err)

	allocs := make([]*common.AntReservation, 0)
	for i := 0; i < 10; i++ {
		alloc, err = mgr.Reserve(ulid.Make().String(), "my-task", "DOCKER", []string{"client-1", "aws"}, "")
		require.NoError(t, err)
		allocs = append(allocs, alloc)
	}

	// WHEN allocating next
	err = mgr.HasAntsForJobTags(
		[]common.TaskMethod{"DOCKER"},
		[]string{"client-1", "aws"},
		"",
	)
	// THEN it should fail
	require.Error(t, err)

	// WHEN allocating next
	err = mgr.HasAntsForJobTags(
		[]common.TaskMethod{"DOCKER"},
		[]string{"client-1", "aws"},
		"",
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
			"",
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
	client, err := queue.NewClientManager().GetClient(context.Background(), &conf.Common)
	require.NoError(t, err)
	mgr := New(conf, client)

	err = mgr.Start(context.Background())
	require.NoError(t, err)

	// WHEN registering with method and tags
	err = registerAnt(
		client,
		conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"DOCKER"},
		[]string{"client-1", "aws", "azure"},
		1)
	require.NoError(t, err)

	allocs := make([]map[string]*common.AntReservation, 0)
	// Each job has two tasks so 10 * 2 but each capacity is matched against requests not tasks
	for i := 0; i < 10; i++ {
		job := newTestJobDefinition(fmt.Sprintf("job-%d", i))
		job.ID = fmt.Sprintf("job-%d", i)
		reservations, err := mgr.ReserveJobResources(job.ID, "", job)
		require.NoError(t, err)
		allocs = append(allocs, reservations)
	}

	// WHEN allocating for non-existing tags
	err = mgr.HasAntsForJobTags(
		[]common.TaskMethod{"DOCKER"},
		[]string{"client-1", "aws"},
		"",
	)
	// THEN it should fail
	require.Error(t, err)

	// reserving resources
	var ids []string
	for i := 0; i < 10; i++ {
		job := newTestJobDefinition(fmt.Sprintf("job-%d", i))
		job.ID = fmt.Sprintf("job-%d", i)
		reservations, err := mgr.ReserveJobResources(job.ID, "", job)
		require.NoError(t, err)
		allocs = append(allocs, reservations)
		ids = append(ids, job.ID)
	}

	// WHEN allocating for non-existing tags
	err = mgr.HasAntsForJobTags(
		[]common.TaskMethod{"DOCKER"},
		[]string{"client-1", "aws"},
		"",
	)
	// THEN it should fail
	require.Error(t, err)

	// releasing
	for _, id := range ids {
		err := mgr.ReleaseJobResources(id)
		if err != nil {
			t.Fatalf("expected allocation %v", err)
		}
		// WHEN allocating after release
		err = mgr.HasAntsForJobTags(
			[]common.TaskMethod{"DOCKER"},
			[]string{"client-1", "aws"},
			"",
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
	client, err := queue.NewClientManager().GetClient(context.Background(), &conf.Common)
	require.NoError(t, err)
	mgr := New(conf, client)
	err = mgr.Start(context.Background())
	require.NoError(t, err)

	// WHEN registering with method and tags
	err = registerAnt(
		client,
		conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"DOCKER"},
		[]string{"client-1", "aws", "azure"},
		1)
	require.NoError(t, err)

	// THEN reservation should fail because `KUBERNETES` method is not available
	_, err = mgr.Reserve(ulid.Make().String(), "task", "KUBERNETES", []string{"client-1", "aws"}, "")
	require.Error(t, err)
}

func Test_ShouldFailReservationWithoutTag(t *testing.T) {
	// GIVEN resource manager is constructed

	testAntID = 0
	conf := config.TestServerConfig()
	err := conf.Validate()
	require.NoError(t, err)
	client, err := queue.NewClientManager().GetClient(context.Background(), &conf.Common)
	require.NoError(t, err)

	mgr := New(conf, client)
	err = mgr.Start(context.Background())
	require.NoError(t, err)

	// WHEN registering with method and tags
	err = registerAnt(
		client,
		conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"DOCKER"},
		[]string{"client-1", "aws", "azure"},
		1)
	require.NoError(t, err)
	err = registerAnt(
		client,
		conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"KUBERNETES"},
		[]string{"client-2", "aws", "google"},
		1)
	require.NoError(t, err)

	// WHEN reservation by tags `DOCKER` and `client-2` is not available
	_, err = mgr.Reserve(ulid.Make().String(), "task", "DOCKER", []string{"client-2", "aws"}, "")
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
	client, err := queue.NewClientManager().GetClient(context.Background(), &conf.Common)
	require.NoError(t, err)
	mgr := New(conf, client)
	err = mgr.Start(context.Background())
	require.NoError(t, err)

	// WHEN registering with method and tags
	err = registerAnt(
		client,
		conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"DOCKER"},
		[]string{"client-1", "aws", "azure"},
		1)
	require.NoError(t, err)
	err = registerAnt(
		client,
		conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"DOCKER", "SHELL"},
		[]string{"client-1", "aws", "azure", "local"},
		1)
	require.NoError(t, err)

	// THEN reservation should succeed up to max-capacity 10
	for i := 0; i < 10; i++ {
		alloc, err := mgr.Reserve(ulid.Make().String(), "my-task", "DOCKER", []string{"client-1", "aws"}, "")
		require.NoError(t, err)
		require.Contains(t, alloc.AntID, "ant-id-") // ant-id-1 or ant-id-2
	}
	require.Equal(t, 2, len(mgr.state.allocationsByAnt))

	// AND reaping allocations should fail because AllocatedAt is recent
	require.Equal(t, 0, mgr.reapStaleAllocations(context.Background()))
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
	client, err := queue.NewClientManager().GetClient(context.Background(), &conf.Common)
	require.NoError(t, err)
	mgr := New(conf, client)
	err = mgr.Start(context.Background())
	require.NoError(t, err)

	// WHEN registering with method and tags
	err = registerAnt(
		client,
		conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"DOCKER"},
		[]string{"client-1", "aws", "azure"},
		1)
	require.NoError(t, err)
	err = registerAnt(
		client,
		conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"DOCKER", "SHELL"},
		[]string{"client-1", "aws", "azure", "local"},
		1)
	require.NoError(t, err)
	err = registerAnt(
		client,
		conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"KUBERNETES", "DOCKER"},
		[]string{"client-1", "aws", "google"},
		6)
	require.NoError(t, err)
	err = registerAnt(
		client,
		conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"KUBERNETES", "DOCKER"},
		[]string{"client-2", "aws", "google"},
		6)
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		// WHEN making reservation
		alloc, err := mgr.Reserve(ulid.Make().String(), "my-task", "DOCKER", []string{"client-1", "aws"}, "")
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
	client, err := queue.NewClientManager().GetClient(context.Background(), &conf.Common)
	require.NoError(t, err)
	mgr := New(conf, client)
	err = mgr.Start(context.Background())
	require.NoError(t, err)

	// WHEN registering with method and tags
	err = registerAnt(
		client,
		conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"DOCKER"},
		[]string{"client-1", "aws", "azure"},
		1)
	require.NoError(t, err)
	err = registerAnt(
		client,
		conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"DOCKER", "SHELL"},
		[]string{"client-1", "aws", "azure", "local"},
		1)
	require.NoError(t, err)
	err = registerAnt(
		client,
		conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"KUBERNETES", "DOCKER"},
		[]string{"client-1", "aws", "google"},
		6)
	require.NoError(t, err)
	err = registerAnt(
		client,
		conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"KUBERNETES", "DOCKER"},
		[]string{"client-2", "aws", "google"},
		6)
	require.NoError(t, err)

	for i := 0; i < 10; i++ {
		// WHEN making reservation
		alloc, err := mgr.Reserve(ulid.Make().String(), "my-task", "DOCKER", []string{"client-1", "aws"}, "")
		// THEN reservation should succeed
		require.NoError(t, err)
		require.Contains(t, alloc.AntID, "ant-id-") // ant-id-1 or ant-id-2
	}

	for i := 0; i < 20; i++ {
		_, err = mgr.Reserve(ulid.Make().String(), "my-task", "DOCKER", []string{"client-1", "aws"}, "")
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
	client, err := queue.NewClientManager().GetClient(context.Background(), &conf.Common)
	require.NoError(t, err)
	mgr := New(conf, client)
	err = mgr.Start(context.Background())
	require.NoError(t, err)

	// WHEN registering with method and tags
	err = registerAnt(
		client,
		conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"DOCKER"},
		[]string{"client-1", "aws", "azure"},
		1)
	require.NoError(t, err)
	err = registerAnt(
		client,
		conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"KUBERNETES"},
		[]string{"client-2", "aws", "google"},
		3)
	require.NoError(t, err)
	err = registerAnt(
		client,
		conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"KUBERNETES"},
		[]string{"client-2", "aws", "google"},
		7)
	require.NoError(t, err)

	// THEN reservation add load
	alloc, err := mgr.Reserve(ulid.Make().String(), "task", "KUBERNETES", []string{"client-2", "aws"}, "")
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
	allocations := make(map[string]*common.AntAllocation)
	antID := fmt.Sprintf("ant-id-%d", testAntID)
	for i := 0; i < load; i++ {
		alloc := &common.AntAllocation{
			JobRequestID: ulid.Make().String(),
			TaskTypes:    map[string]common.RequestState{"task": common.EXECUTING},
			AntID:        antID,
			AllocatedAt:  time.Now(),
		}
		allocations[alloc.JobRequestID] = alloc
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
			b,
			make(map[string]string),
		)
	}
	return
}

// registerAntWithOrg registers an ant with an org scoping for unit tests.
// It sets OrgID in both the AntRegistration JSON body and the AntOrgIDHeader property.
//
// In production the queen ignores OrgID in the JSON body and stamps it exclusively
// from the server-side AntOrgIDHeader (populated from the JWT at WebSocket connect time).
// This helper sets both because unit tests bypass the WebSocket auth layer — the
// subscription callback reads the header and overwrites the body value, so the
// double-set is idempotent and does not validate that ants can self-report their org.
func registerAntWithOrg(
	queueClient queue.Client,
	registrationTopic string,
	methods []common.TaskMethod,
	tags []string,
	orgID string,
	load int) (err error) {
	testAntID++
	allocations := make(map[string]*common.AntAllocation)
	antID := fmt.Sprintf("ant-id-%d", testAntID)
	for i := 0; i < load; i++ {
		alloc := &common.AntAllocation{
			JobRequestID: ulid.Make().String(),
			TaskTypes:    map[string]common.RequestState{"task": common.EXECUTING},
			AntID:        antID,
			AllocatedAt:  time.Now(),
		}
		allocations[alloc.JobRequestID] = alloc
	}
	registration := common.AntRegistration{
		AntID:       antID,
		MaxCapacity: 10,
		Tags:        tags,
		Methods:     methods,
		Allocations: allocations,
		OrgID:       orgID,
	}
	registration.AntTopic = testIncomingTopic
	registration.CurrentLoad = load
	var b []byte
	if b, err = registration.Marshal(); err == nil {
		props := make(map[string]string)
		if orgID != "" {
			props[queue.AntOrgIDHeader] = orgID
		}
		_, err = queueClient.Publish(
			context.Background(),
			registrationTopic,
			b,
			props,
		)
	}
	return
}

func Test_Should_Route_Job_To_OrgScoped_Ant_First(t *testing.T) {
	// GIVEN antA(OrgID="org-1") and antB(OrgID="") both support SHELL
	testAntID = 0
	conf := config.TestServerConfig()
	require.NoError(t, conf.Validate())
	client, err := queue.NewClientManager().GetClient(context.Background(), &conf.Common)
	require.NoError(t, err)
	mgr := New(conf, client)
	require.NoError(t, mgr.Start(context.Background()))

	err = registerAntWithOrg(client, conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"SHELL"}, []string{}, "org-1", 0)
	require.NoError(t, err)
	err = registerAntWithOrg(client, conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"SHELL"}, []string{}, "", 0)
	require.NoError(t, err)

	// WHEN reserving with orgID="org-1"
	alloc, err := mgr.Reserve(ulid.Make().String(), "task", "SHELL", []string{}, "org-1")
	// THEN org-scoped ant is preferred
	require.NoError(t, err)
	reg := mgr.state.antRegistrations[alloc.AntID]
	require.Equal(t, "org-1", reg.OrgID)

	require.NoError(t, mgr.Stop(context.Background()))
}

func Test_Should_Fallback_To_Unscoped_Ant_When_No_Org_Ant_Alive(t *testing.T) {
	// GIVEN only antA(OrgID="") is available; no org-scoped ant
	testAntID = 0
	conf := config.TestServerConfig()
	require.NoError(t, conf.Validate())
	client, err := queue.NewClientManager().GetClient(context.Background(), &conf.Common)
	require.NoError(t, err)
	mgr := New(conf, client)
	require.NoError(t, mgr.Start(context.Background()))

	err = registerAntWithOrg(client, conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"SHELL"}, []string{}, "", 0)
	require.NoError(t, err)

	// WHEN reserving with orgID="org-1" but no org-1 ant exists
	alloc, err := mgr.Reserve(ulid.Make().String(), "task", "SHELL", []string{}, "org-1")
	// THEN unscoped ant is used as fallback
	require.NoError(t, err)
	reg := mgr.state.antRegistrations[alloc.AntID]
	require.Equal(t, "", reg.OrgID)

	require.NoError(t, mgr.Stop(context.Background()))
}

func Test_Should_Not_Route_Job_To_Different_Org_Ant(t *testing.T) {
	// GIVEN antA(OrgID="org-1") and antB(OrgID="org-2"), no unscoped ant
	testAntID = 0
	conf := config.TestServerConfig()
	require.NoError(t, conf.Validate())
	client, err := queue.NewClientManager().GetClient(context.Background(), &conf.Common)
	require.NoError(t, err)
	mgr := New(conf, client)
	require.NoError(t, mgr.Start(context.Background()))

	err = registerAntWithOrg(client, conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"SHELL"}, []string{}, "org-1", 0)
	require.NoError(t, err)
	err = registerAntWithOrg(client, conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"SHELL"}, []string{}, "org-2", 0)
	require.NoError(t, err)

	// WHEN reserving with orgID="org-1"
	alloc, err := mgr.Reserve(ulid.Make().String(), "task", "SHELL", []string{}, "org-1")
	// THEN only org-1 ant is chosen
	require.NoError(t, err)
	reg := mgr.state.antRegistrations[alloc.AntID]
	require.Equal(t, "org-1", reg.OrgID)

	require.NoError(t, mgr.Stop(context.Background()))
}

func Test_Should_Be_Noop_When_Auth_Disabled(t *testing.T) {
	// GIVEN two ants with no OrgID (auth disabled)
	testAntID = 0
	conf := config.TestServerConfig()
	require.NoError(t, conf.Validate())
	client, err := queue.NewClientManager().GetClient(context.Background(), &conf.Common)
	require.NoError(t, err)
	mgr := New(conf, client)
	require.NoError(t, mgr.Start(context.Background()))

	err = registerAntWithOrg(client, conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"SHELL"}, []string{}, "", 0)
	require.NoError(t, err)
	err = registerAntWithOrg(client, conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"SHELL"}, []string{}, "", 0)
	require.NoError(t, err)

	// WHEN reserving with orgID="" (auth disabled)
	alloc, err := mgr.Reserve(ulid.Make().String(), "task", "SHELL", []string{}, "")
	// THEN either ant can be chosen without error
	require.NoError(t, err)
	require.NotEmpty(t, alloc.AntID)

	require.NoError(t, mgr.Stop(context.Background()))
}

func Test_Should_Apply_Org_Filter_Before_Method_Filter(t *testing.T) {
	// GIVEN antA(OrgID="org-1", method=DOCKER), antB(OrgID="org-2", method=SHELL)
	// no unscoped ant
	testAntID = 0
	conf := config.TestServerConfig()
	require.NoError(t, conf.Validate())
	client, err := queue.NewClientManager().GetClient(context.Background(), &conf.Common)
	require.NoError(t, err)
	mgr := New(conf, client)
	require.NoError(t, mgr.Start(context.Background()))

	err = registerAntWithOrg(client, conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"DOCKER"}, []string{}, "org-1", 0)
	require.NoError(t, err)
	err = registerAntWithOrg(client, conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"SHELL"}, []string{}, "org-2", 0)
	require.NoError(t, err)

	// WHEN reserving org-1 with SHELL (org-1 ant only supports DOCKER)
	_, err = mgr.Reserve(ulid.Make().String(), "task", "SHELL", []string{}, "org-1")
	// THEN no ant found — org-1 ant doesn't support SHELL, org-2 ant excluded by org filter
	require.Error(t, err)

	require.NoError(t, mgr.Stop(context.Background()))
}

func Test_Should_Fallback_To_Unscoped_When_OrgAnt_Goes_Stale(t *testing.T) {
	// GIVEN antA(OrgID="org-1") registered but goes stale, antB(OrgID="") is live
	testAntID = 0
	conf := config.TestServerConfig()
	require.NoError(t, conf.Validate())
	client, err := queue.NewClientManager().GetClient(context.Background(), &conf.Common)
	require.NoError(t, err)
	mgr := New(conf, client)
	require.NoError(t, mgr.Start(context.Background()))

	err = registerAntWithOrg(client, conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"SHELL"}, []string{}, "org-1", 0)
	require.NoError(t, err)
	err = registerAntWithOrg(client, conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"SHELL"}, []string{}, "", 0)
	require.NoError(t, err)

	// Force the org-1 ant to appear stale (ReceivedAt far in the past)
	for id, reg := range mgr.state.antRegistrations {
		if reg.OrgID == "org-1" {
			mgr.state.antRegistrations[id].ReceivedAt = time.Unix(0, 0)
		}
	}

	// WHEN reserving with orgID="org-1" — org ant is stale, unscoped ant is live
	alloc, err := mgr.Reserve(ulid.Make().String(), "task", "SHELL", []string{}, "org-1")
	// THEN fallback to unscoped ant succeeds (not a "no ants could be reserved" error)
	require.NoError(t, err)
	reg := mgr.state.antRegistrations[alloc.AntID]
	require.Equal(t, "", reg.OrgID)

	require.NoError(t, mgr.Stop(context.Background()))
}

func Test_Should_Remove_Org_Index_On_Ant_Removal(t *testing.T) {
	// GIVEN antA registered with OrgID="org-1"
	testAntID = 0
	conf := config.TestServerConfig()
	require.NoError(t, conf.Validate())
	client, err := queue.NewClientManager().GetClient(context.Background(), &conf.Common)
	require.NoError(t, err)
	mgr := New(conf, client)
	require.NoError(t, mgr.Start(context.Background()))

	err = registerAntWithOrg(client, conf.Common.GetRegistrationTopic(),
		[]common.TaskMethod{"SHELL"}, []string{}, "org-1", 0)
	require.NoError(t, err)

	// Capture the ant ID
	var antID string
	for id := range mgr.state.antRegistrations {
		antID = id
	}
	require.NotEmpty(t, antID)
	require.Contains(t, mgr.state.antsByOrg["org-1"], antID)

	// WHEN ant is unregistered
	_, err = mgr.Unregister(context.Background(), antID)
	require.NoError(t, err)

	// THEN antsByOrg index is cleaned up
	require.NotContains(t, mgr.state.antsByOrg["org-1"], antID)

	require.NoError(t, mgr.Stop(context.Background()))
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
