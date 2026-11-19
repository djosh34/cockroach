# The Job System

Original authors: Michael Butler & Shiranka Miskin (November 2021)



## What is a Job?

Jobs are CockroachDB's way of representing long running background tasks.  They
are used internally as a core part of various features of CockroachDB such as
Imports, Schema Changes and statistics collection.  Progress is regularly persisted such
that a node may fail / the job can be paused and a node will still be able to
resume the work later on.


## User Control

Users can send `PAUSE`/`RESUME`/`CANCEL` commands to specific jobs via SQL
commands such as `PAUSE JOB {job_id}` for a single job, to an arbitrary set of
jobs with commands such as `CANCEL JOBS {select_clause}`, to specific job types
through commands such as `RESUME ALL IMPORT JOBS`, or to jobs triggered by a
given [schedule](##scheduled-jobs) through commands such as `PAUSE JOBS FOR
SCHEDULE {schedule_id}`.

Commands to specific job ids are handled through
[`controlJobsNode.startExec`](https://github.com/cockroachdb/cockroach/blob/ae17f3df3448dcf13d4b187f1c45256cfa17d2f7/pkg/sql/control_jobs.go#L113).
`PAUSE` and `CANCEL` commands move the job to a `pause-requested` and
`cancel-requested` state respectively, which allows the node currently running
the job to proceed with actually pausing or cancelling it (see `jobs/cancel` in
[Job Management](#job-management)).  `RESUME` moves the job to a `running` state
to be picked up by any node (see `jobs/adopt` in [Job
Management](#job-management)).

The batch commands based on a schedule / type are simply delegated to the
respective `{cmd} JOBS` command via
[`delegateJobControl`](https://github.com/cockroachdb/cockroach/blob/ae17f3df3448dcf13d4b187f1c45256cfa17d2f7/pkg/sql/delegate/job_control.go#L40).


## Internal Representation

A Job is represented as a row in the
[`system.jobs`](https://github.com/cockroachdb/cockroach/blob/7097a9015f1a09c7dee4fbdbcc6bde82121f657b/pkg/sql/catalog/systemschema/system.go#L185)
table which is the single source of truth for individual job information. Nodes
read this table to determine the jobs they can execute, using the
[`payload`](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/jobspb/jobs.proto#L755)
to determine how to execute them and the
[`progress`](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/jobspb/jobs.proto#L815)
to pick up from where other nodes may have left it. Completed/cancelled jobs are
[eventually](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/config.go#L107)
garbage collected from `system.jobs`.

```sql
CREATE TABLE system.jobs (
  id                INT8      DEFAULT unique_rowid(),
  created           TIMESTAMP NOT NULL DEFAULT now(),

  -- States such as "pending", "running", "paused", "pause-requested", etc.
  status            STRING    NOT NULL,

  -- Information specific to each type of job (ex: Import, Schema Change).
  payload           BYTES     NOT NULL,   -- inspectable via crdb_internal.pb_to_json('cockroach.sql.jobs.jobspb.Payload', payload)
  progress          BYTES,                -- inspectable via crdb_internal.pb_to_json('cockroach.sql.jobs.jobspb.Progress', progress)

  -- Used to track which node currently owns execution of the job
  claim_session_id  BYTES,
  claim_instance_id INT8,

  -- The system that created the job and the id relevant to that system (ex: crdb_schedule and the schedule_id)
  created_by_type   STRING,
  created_by_id     INT,

  -- Useful for observability
  num_runs          INT8,
  last_run          TIMESTAMP,

  ... -- constraint/index/family
)
```

Jobs are primarily specified and differentiated through their `payload` column
which are bytes of type
[jobspb.Payload](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/jobspb/jobs.proto#L755),
storing information related to the type, parameters, and metadata of the job.

```protobuf
message Payload {
  // General information relevant to any job
  string description = 1;
  repeated string statement = 16;
  int64 started_micros = 3;
  int64 finished_micros = 4;
  ...
  repeated errorspb.EncodedError resume_errors = 17;
  repeated errorspb.EncodedError cleanup_errors = 18;
  errorspb.EncodedError final_resume_error = 19;
  ...
  // Type-specific payload details that mark the type of job and stores type-specific information
  oneof details {
    BackupDetails backup = 10;
    RestoreDetails restore = 11;
    ... // Details for the many other types of jobs
  }
  ...
}

```



The progress of the specified job through its lifecycle is tracked in a separate
`progress` column of type
[`jobspb.Progress`](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/jobspb/jobs.proto#L815).

```protobuf
message Progress {
  // Generic information for observability
  oneof progress {
    float fraction_completed = 1;
    util.hlc.Timestamp high_water = 3;
  }
  string running_status = 4;

  // Type-specific progress information similar to Payload details
  oneof details { // Note that while this is also called details, it does not take {JobType}Details structs like Payload
    BackupProgress backup = 10;
    RestoreProgress restore = 11;
    ... // Progress for the many other types of jobs
  }
  ...
}
```

A more user-readable version of `system.jobs` with the parsed `payload` and
`progress` information can be monitored through the
[`crdb_internal.jobs`](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/sql/crdb_internal.go#L660)
table (which simply reads from `system.jobs`).  The [`SHOW
JOBS`](https://www.cockroachlabs.com/docs/stable/show-jobs.html) command
operates by [reading from
`crdb_internal.jobs`](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/sql/delegate/show_jobs.go#L23).

A Job is created when a [new row is
inserted](https://github.com/cockroachdb/cockroach/blob/7097a9015f1a09c7dee4fbdbcc6bde82121f657b/pkg/jobs/registry.go#L550)
to `system.jobs`.  Once written, it is able to be "claimed" by any node in the
cluster (asserting that it is responsible for executing the job) through the
[setting of `claim_session_id` and
`claim_instance_id`](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/adopt.go#L48)
in the job record.  Job execution ends when the status changes from `running` to
`success`, `cancelled`, or `failure`. The job record eventually gets
[deleted](https://github.com/cockroachdb/cockroach/blob/7097a9015f1a09c7dee4fbdbcc6bde82121f657b/pkg/jobs/registry.go#L1000)
from the system table -- by default, 14 hours after execution ends.

If that node fails to completely execute the job (either the node failing or the
job being paused), once the job record is in the table it is also able to be
resumed by any node in the cluster. The mechanism by which this is done is the
[`JobRegistry`](#Job-Management).


## The Job Registry
A node interacts with the jobs table through the [`JobRegistry`](https://github.com/cockroachdb/cockroach/blob/7097a9015f1a09c7dee4fbdbcc6bde82121f657b/pkg/jobs/registry.go#L92)
struct.


### Job Creation
A node creates a job by calling the Registry's
[`CreateJobWithTxn`](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/registry.go#L523)
or
[`CreateAdoptableJobWithTxn`](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/registry.go#L560)
which
[`INSERT`s](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/registry.go#L550)
a new row into `system.jobs` . The node passes the job specific
information to these functions via the
[`Record`](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/jobs.go#L91)
struct.

If the node created the job with `CreateJobWithTxn`, it will also claim the job by setting
the claim IDs and
[start](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/jobs.go#L890)
the job. By contrast, `CreateAdoptableJobWithTxn` allows another node to adopt and
[resume](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/adopt.go#L246)
the job (`startableJob.Start()` is never called on these jobs).


### Job Management
While job creation is triggered in response to client queries such as
`IMPORT`, job adoption, cancellation, and deletion is managed
through daemon goroutines. These daemon goroutines will continually run
on each node, allowing any node to participate in picking up work, unassigning
jobs that were on nodes that failed, and cleaning up old job records from the
table.

These goroutines begin when each node's SQL Server
[starts](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/server/server.go#L1876),
initializing the `JobRegistry` via
[`jobRegistry.Start(...)`](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/server/server_sql.go#L1083).
Specifically, `jobRegistry.Start()` kicks off the following goroutines:

- **[`jobs/adopt`](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/registry.go#L896)**
  : At the
  [`jobs.registry.interval.adopt`](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/config.go#L82)
  interval (default 30s), call
  [`claimJobs`](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/adopt.go#L91)
  to poll (query?) `system.jobs` and attempt to
  [`UPDATE`](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/adopt.go#L48)
  up to `$COCKROACH_JOB_ADOPTIONS_PER_PERIOD` (default 10) rows without an
  assigned `claim_session_id` with the current node's session and instance IDs.
  After [claiming](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/adopt.go#L48)
  new jobs, the node [resumes them](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/adopt.go#L194).


- **[`jobs/cancel`](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/registry.go#L839)**
  : At the
  [`jobs.registry.interval.cancel`](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/config.go#L91)
  interval (default 10s), [set `claim_session_id =
  NULL`](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/registry.go#L749)
  for up to
  [`jobs.cancel_update_limit`](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/config.go#L114)
  jobs with session IDs that fail a [SQL Liveness
  check](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/sql/sqlliveness/slstorage/slstorage.go#L190).
  See the [SQL Liveness RFC](https://github.com/cockroachdb/cockroach/blob/master/docs/RFCS/20200615_sql_liveness.md)
  for more information on liveness and claim IDs.  Jobs claimed by the current node in the `pause-requested`
  and `cancel-requested` states are also
  [transitioned](https://github.com/cockroachdb/cockroach/blob/ae17f3df3448dcf13d4b187f1c45256cfa17d2f7/pkg/jobs/adopt.go#L430)
  to the `PAUSED` and `REVERTING` states by the [same daemon
  goroutine](https://github.com/cockroachdb/cockroach/blob/ae17f3df3448dcf13d4b187f1c45256cfa17d2f7/pkg/jobs/registry.go#L815).

- **[`jobs/gc`](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/registry.go#L863)**
  : At the
  [`jobs.registry.interval.gc`](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/config.go#L99)
  interval (default 1h), query for jobs with a `status` of
  [Succeded/Cancelled/Failed](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/registry.go#L983)
  that stopped running more than
  [`jobs.retention_time`](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/config.go#L107)
  (default 2 weeks) ago and
  [`DELETE`](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/registry.go#L1000)
  them.
  - Since the finished time is stored in the `finished_micros` of the
    `Payload` protobuf and cannot be read directly by SQL, up to
    [100](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/registry.go#L935)
    job records are
    [`SELECT`](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/registry.go#L951) ed
    and their `payload`s are
    [unmarshalled](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/registry.go#L977)
    to filter on the finished time.


### Job Execution

Each job type registers its respective execution logic through
[jobs.RegisterConstructor](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/registry.go#L1153)
which globally registers a
[`Constructor`](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/registry.go#L1148)
function for a given
[`jobspb.Type`](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/jobspb/jobs.pb.go#L103).
This `Constructor` returns an implementation of
[`jobs.Resumer`](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/registry.go#L1111)
with the `Resume` and `OnFailOrCancel` functions for the registry to execute.

When a job is adopted and
[resumed](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/adopt.go#L246)
by a node's registry, unless the job is not already running or completed,
[runJob](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/adopt.go#L367)
is ran in its own goroutine as an [async
task](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/adopt.go#L333).
During adoption a [new context is
created](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/registry.go#L662)
for the `runJob` execution with its own
[`cancel`](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/registry.go#L259)
function that is stored in the
registry's internal
[map](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/registry.go#L143)
of [adopted
jobs](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/registry.go#L52).
This cancel callback stored in the registry allows it to remotely terminate the
job when it must
[`servePauseAndCancelRequests`](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/adopt.go#L443).

This thread handles executing the job by modeling it as a state machine through
the registry's
[`stepThroughStateMachine`](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/registry.go#L1174),
starting from the `StatusRunning` state.  It [calls
resumer.Resume](https://github.com/cockroachdb/cockroach/blob/8a501a247f177bf287bcf34beb4f05155818998c/pkg/jobs/registry.go#L1209)
to execute the job-specific logic, and once the function completes it
recursively calls `stepThroughStateMachine` to transition to the appropriate
next state depending on if the resumption completed successfully or due to some
form of error (either through the job's own execution or through a context error
triggered by `servePauseAndCancelRequests`).  As it transitions through the
state machine it updates job state accordingly and potentially calls
`resumer.OnFailOrCancel` for job-specific handling.  Eventually the original
`stepThroughStateMachine` exits and the `runJob` thread can complete.




## Scheduled Jobs

Jobs on their own are triggered by specific user commands such as `IMPORT` or
schema changes, however cockroachdb also supports scheduling jobs to run in
the future and be able to recur based on a crontab schedule. Schedules are used
internally, for example for SQL stats compaction and row-level TTL.

Similar to Jobs, Schedules can be `PAUSE`d, `RESUME`d, or `DROP`ped via `{cmd}
SCHEDULES {select_clause}` and are handled by
[`controlSchedulesNode.startExec`](https://github.com/cockroachdb/cockroach/blob/ae17f3df3448dcf13d4b187f1c45256cfa17d2f7/pkg/sql/control_schedules.go#L136).
`PAUSE` and `RESUME` simply set the `next_run` of the schedule to either
an empty value or the next iteration according to the `schedule_expr`, while
`CANCEL` deletes the record from the table.

Job Schedules are stored in the
[`system.scheduled_jobs`](https://github.com/cockroachdb/cockroach/blob/ae17f3df3448dcf13d4b187f1c45256cfa17d2f7/pkg/sql/catalog/systemschema/system.go#L432)
table. A more user-readable version can be viewed using the [`SHOW
SCHEDULES`](https://www.cockroachlabs.com/docs/stable/show-schedules.html) SQL
statement.

```sql
CREATE TABLE system.scheduled_jobs (
    schedule_id      INT DEFAULT unique_rowid() NOT NULL,
    schedule_name    STRING NOT NULL,
    created          TIMESTAMPTZ NOT NULL DEFAULT now(),
    owner            STRING NOT NULL,
    next_run         TIMESTAMPTZ, -- the next scheduled run of the job in UTC, NULL if paused
    schedule_state   BYTES,       -- inspectable via crdb_internal.pb_to_json('cockroach.jobs.jobspb.ScheduleState', schedule_state) 
    schedule_expr    STRING,      -- job schedule in crontab format, if empty the schedule will not recur
    schedule_details BYTES,       -- inspectable via crdb_internal.pb_to_json('cockroach.jobs.jobspb.ScheduleDetails', schedule_details)
    executor_type    STRING NOT NULL,
    execution_args   BYTES NOT NULL,
    ...
)
```

State on the schedule's behavior and current status are stored in the
`schedule_details` and `schedule_state` columns respectively which follow the
following protobuf formats:

```protobuf
// How to schedule and execute the job.
message ScheduleDetails {
  // How to handle encountering a previously started job that hasn't completed yet
  enum WaitBehavior {
    WAIT = 0; // Wait for previous run to complete then run
    NO_WAIT = 1; // Do not wait and just run a potentially overlapping execution
    SKIP = 2; // If the previous run is ongoing, just advance the schedule without running
  }

  // How to proceed if the scheduled job fails
  enum ErrorHandlingBehavior {
    RETRY_SCHED = 0; // Allow the job to execute again next time its scheduled to do so
    RETRY_SOON = 1; // Retry the job almost immediately after failure
    PAUSE_SCHED = 2; // Pause the schedule entirely
  }

  WaitBehavior wait = 1;
  ErrorHandlingBehavior on_error = 2;
}

// Mutable state for the schedule such as error strings
message ScheduleState {
  string status = 1;
}
```

A CRDB node writes to a row in the scheduled jobs table through the [`ScheduledJob`](https://github.com/cockroachdb/cockroach/blob/ae17f3df3448dcf13d4b187f1c45256cfa17d2f7/pkg/jobs/scheduled_job.go#L55)
struct by queuing up individual changes in its `dirty` property and then either [creating](https://github.com/cockroachdb/cockroach/blob/ae17f3df3448dcf13d4b187f1c45256cfa17d2f7/pkg/jobs/scheduled_job.go#L365)
or [updating](https://github.com/cockroachdb/cockroach/blob/ae17f3df3448dcf13d4b187f1c45256cfa17d2f7/pkg/jobs/scheduled_job.go#L398)
a row in the table to commit them. Next, we'll discuss when a CRDB node would write to this table.


### Schedule Creation

A [`ScheduledJob`](https://github.com/cockroachdb/cockroach/blob/ae17f3df3448dcf13d4b187f1c45256cfa17d2f7/pkg/jobs/scheduled_job.go#L55) struct is first created via
[`NewScheduledJob`](https://github.com/cockroachdb/cockroach/blob/ae17f3df3448dcf13d4b187f1c45256cfa17d2f7/pkg/jobs/scheduled_job.go#L72),
initializing the struct which can then have its properties be set using the
setter functions.  Finally, the data in the struct is persisted into the
`scheduled_jobs` table via
[`ScheduledJob.Create`](https://github.com/cockroachdb/cockroach/blob/ae17f3df3448dcf13d4b187f1c45256cfa17d2f7/pkg/jobs/scheduled_job.go#L365).


### Schedule Management


When a SQLServer initializes, it calls
[`StartJobSchedulerDaemon`](https://github.com/cockroachdb/cockroach/blob/ae17f3df3448dcf13d4b187f1c45256cfa17d2f7/pkg/server/server_sql.go#L1163)
to start the
[`job-scheduler`](https://github.com/cockroachdb/cockroach/blob/ae17f3df3448dcf13d4b187f1c45256cfa17d2f7/pkg/jobs/job_scheduler.go#L355)
async task, similar to other job background tasks.  Unless
[`jobs.scheduler.enabled`](https://github.com/cockroachdb/cockroach/blob/ae17f3df3448dcf13d4b187f1c45256cfa17d2f7/pkg/jobs/job_scheduler.go#L387)
is false, on an interval of
[`jobs.scheduler.pace`](https://github.com/cockroachdb/cockroach/blob/ae17f3df3448dcf13d4b187f1c45256cfa17d2f7/pkg/jobs/job_scheduler.go#L393)
(default 1 minute), the node will attempt to
[process](https://github.com/cockroachdb/cockroach/blob/ae17f3df3448dcf13d4b187f1c45256cfa17d2f7/pkg/jobs/job_scheduler.go#L132)
up to
[`jobs.scheduler.max_jobs_per_iteration`](https://github.com/cockroachdb/cockroach/blob/ae17f3df3448dcf13d4b187f1c45256cfa17d2f7/pkg/jobs/job_scheduler.go#L399)
schedules (default 10) with a `next_run` timestamp earlier than the current time.


If no jobs have successfully started for this current execution of the schedule,
a type-specific executor will be called to queue the appropriate jobs to be
executed.

Once jobs created by this schedule are observed, the `next_run` of the schedule
will be advanced by
[`processSchedule`](https://github.com/cockroachdb/cockroach/blob/ae17f3df3448dcf13d4b187f1c45256cfa17d2f7/pkg/jobs/job_scheduler.go#L132)
according to the `schedule_expr` with some added jitter to avoid conflicting
transactions.

The executor for a given type of Job Schedule is registered via the
[`RegisterScheduledJobExecutorFactory`](https://github.com/cockroachdb/cockroach/blob/ae17f3df3448dcf13d4b187f1c45256cfa17d2f7/pkg/jobs/scheduled_job_executor.go#L81)
function.
The factory must return a struct that implements the
[ScheduledJobsExecutor](https://github.com/cockroachdb/cockroach/blob/ae17f3df3448dcf13d4b187f1c45256cfa17d2f7/pkg/jobs/scheduled_job_executor.go#L26)
interface.

```go
type ScheduledJobExecutor interface {
	ExecuteJob(...) error
	NotifyJobTermination(...) error
	Metrics() metric.Struct
	GetCreateScheduleStatement(...) (string, error)
}
```

NB: Jobs created by a schedule will have their `created_by_type` and `created_by_id`
columns set to that of the schedule that created them.


# Applications of the Job System 
In this section, we detail the code paths taken by CRDB SQL statements that spawn jobs, such as 
IMPORT and Schema Changes (e.g. ALTER). 

## Job Planning
SQL Statments that rely on the job system begin to branch from the conventional
life of a query during [Logical Planning and Optimization](https://github.com/cockroachdb/cockroach/blob/master/docs/tech-notes/life_of_a_query.md#logical-planning-and-optimization)
(read through this section first!). Specifically, after following a few internal calls from
`makeExecPlan`, we call [`trybuildOpaque`](https://github.com/cockroachdb/cockroach/blob/df67aa9707fbf0193ec8b3ca4062240c360fc808/pkg/sql/opt/optbuilder/opaque.go#L64)
to convert an AST into a memo. 
 - Detailed Note: `tryBuildOpaque` will match the statement to one
of the opaqueStatements, a map populated by [Opaque.go’s `init()`](https://github.com/cockroachdb/cockroach/blob/31db44da69bf21e67ebbef6fbb8c8bfb2e498efe/pkg/sql/opaque.go#L296)
, and then, call
[`buildOpaque`](https://github.com/cockroachdb/cockroach/blob/31db44da69bf21e67ebbef6fbb8c8bfb2e498efe/pkg/sql/opaque.go#L38)
mysteriously [here](https://github.com/cockroachdb/cockroach/blob/df67aa9707fbf0193ec8b3ca4062240c360fc808/pkg/sql/opt/optbuilder/opaque.go#L69). 
Each kind of job will take a different path through `buildOpaque`, which we’ll discuss in more 
detail later. 

Further, unlike the normal logical planning path for SQL queries, 
`tryBuildOpaque` [skips](https://github.com/cockroachdb/cockroach/blob/df67aa9707fbf0193ec8b3ca4062240c360fc808/pkg/sql/opt/optbuilder/opaque.go#L75) 
Cockroach’s optimization engine, and returns a populated `memo` object. Why skip query
optimization?? Cockroach SQL statements that spawn jobs don’t contain any
subqueries or additional operators that would benefit from query optimization
(e.g. JOIN, SELECT, WHERE).

Finally, just like any regular query, the job’s memo object is then
[converted](https://github.com/cockroachdb/cockroach/blob/df67aa9707fbf0193ec8b3ca4062240c360fc808/pkg/sql/plan_opt.go#L262)
(specifically
[here](https://github.com/cockroachdb/cockroach/blob/fc67a0c9202af348e919afc1e1f70acc9a83b300/pkg/sql/opt/exec/execbuilder/relational.go#L2355))
into a query plan, and
[`execWithDistSQLEngine`](https://github.com/cockroachdb/cockroach/blob/fc67a0c9202af348e919afc1e1f70acc9a83b300/pkg/sql/conn_executor_exec.go#L925-L927)
starts the job, specifically via spawning a new goroutine
[here](https://github.com/cockroachdb/cockroach/blob/df67aa9707fbf0193ec8b3ca4062240c360fc808/pkg/sql/planhook.go#L139)
(**not sure if all jobs go through hookfn node**). The exact function run by the
go routine varies by job. Note, unlike many other [query
plans](https://github.com/cockroachdb/cockroach/blob/master/docs/tech-notes/life_of_a_query.md#physical-planning-and-execution)
, DistSQLEngine will not create a distributed physical plan for the job--which
you can verify by checking the
[`distributePlan`](https://github.com/cockroachdb/cockroach/blob/fc67a0c9202af348e919afc1e1f70acc9a83b300/pkg/sql/conn_executor_exec.go#L916)
variable-- rather, each specific job will manually distribute work across nodes
during execution. Next we'll discuss how certain CRDB features spawn jobs in the DistSQL Engine. 

### Schema Changes Job Planning
TODO (hopefully by someone on SQL Schema :))))

## Job Execution
Next, we describe how various kinds of jobs work, i.e. when a node picks up a job, what does it
actually do? This will be more high level, and will point towards RFCs/Docs/Blogposts for further
details.
TODO(???): Schema Changes
