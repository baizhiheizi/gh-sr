package runner

import _ "embed"

//go:embed svc.sh
var SVCShContent string

//go:embed actions.runner.service.template
var ServiceTemplateContent string
