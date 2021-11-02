// Package docs Formicary API.
//
// The formicary is a distributed orchestration engine based on `Leader-Follower`, `Pipes-Filter`, `Fork-Join` and `SEDA` design principles for
// executing a directed acyclic graph of tasks, which is also referred as a job workflow. A task represents a unit of work and a job definition is used to specify the task
// dependencies in the graph/workflow including configuration parameters and conditional logic.
//
//     Schemes: http, https
//     Host: https://formicary.io
//     BasePath: /api
//     Version: 0.0.1
//     License: AGPL https://opensource.org/licenses/AGPL-3.0
//     Contact: Support<support@formicary.io> https://formicary.io
//
//     Consumes:
//     - application/json
//
//     Produces:
//     - application/json
//
//     Security:
//     - api_key:
//
//     SecurityDefinitions:
//     api_key:
//          type: apiKey
//          name: Authorization
//          in: header
//
//     Extensions:
//     x-meta-value: value
//
// swagger:meta
package docs
