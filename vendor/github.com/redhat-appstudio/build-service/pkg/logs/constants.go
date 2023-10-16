/*
Copyright 2023 Red Hat, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package logs

const (
	Action = "action"
	Audit  = "audit"
)

// Action type represents all possible value of 'action' log field.
// For more details see https://github.com/redhat-appstudio/book/blob/main/ADR/0006-log-conventions.md
type ActionLogValue string

const (
	ActionView   ActionLogValue = "VIEW"
	ActionAdd    ActionLogValue = "ADD"
	ActionUpdate ActionLogValue = "UPDATE"
	ActionDelete ActionLogValue = "DELETE"
)
