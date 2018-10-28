package api

import (
	"github.com/mlowicki/rhythm/model"

	"github.com/xeipuuv/gojsonschema"
)

type cronFormatChecker struct{}

func (f cronFormatChecker) IsFormat(input interface{}) bool {
	cron, ok := input.(string)
	if !ok {
		return false
	}
	_, err := model.CronParser.Parse(cron)
	return err == nil
}

func init() {
	gojsonschema.FormatCheckers.Add("cron", cronFormatChecker{})
}

const (
	groupPattern   = "^[a-zA-Z0-9-_]+$"
	projectPattern = groupPattern
	jobIDPattern   = groupPattern
)

type schema map[string]interface{}

type newJobPayload struct {
	Group    string
	Project  string
	ID       string
	Schedule struct {
		Cron string
	}
	Env       map[string]string
	Secrets   map[string]string
	Container struct {
		Docker struct {
			Image          string
			ForcePullImage bool
		}
		Mesos struct {
			Image string
		}
	}
	CPUs      float64
	Mem       float64
	Disk      float64
	Cmd       string
	User      string
	Shell     *bool
	Arguments []string
	Labels    map[string]string
}

var newJobSchema = schema{
	"type": "object",
	"properties": schema{
		"Group": schema{
			"type":    "string",
			"pattern": groupPattern,
		},
		"Project": schema{
			"type":    "string",
			"pattern": projectPattern,
		},
		"ID": schema{
			"type":    "string",
			"pattern": jobIDPattern,
		},
		"Schedule": schema{
			"type": "object",
			"properties": schema{
				"Cron": schema{
					"type":   "string",
					"format": "cron",
				},
			},
			"required": []string{"Cron"},
		},
		"Container": schema{
			"type": "object",
			"oneOf": []schema{
				schema{
					"properties": schema{
						"Docker": schema{
							"type": "object",
							"properties": schema{
								"Image": schema{
									"type":      "string",
									"minLength": 1,
								},
							},
						},
					},
				},
				schema{
					"properties": schema{
						"Mesos": schema{
							"type": "object",
							"properties": schema{
								"Image": schema{
									"type":      "string",
									"minLength": 1,
								},
							},
						},
					},
				},
			},
		},
		"CPUs": schema{
			"type":             "number",
			"minimum":          0,
			"exclusiveMinimum": true,
		},
		"Disk": schema{
			"type":             "number",
			"minimum":          0,
			"exclusiveMinimum": true,
		},
		"Mem": schema{
			"type":             "number",
			"minimum":          0,
			"exclusiveMinimum": true,
		},
	},
	"required": []string{"Group", "Project", "ID", "Schedule", "Mem", "CPUs", "Disk"},
}

type updateJobPayload struct {
	Schedule *struct {
		Cron *string
	}
	Env       *map[string]string
	Secrets   *map[string]string
	Container *struct {
		Docker *struct {
			Image          *string
			ForcePullImage *bool
		}
		Mesos *struct {
			Image *string
		}
	}
	CPUs      *float64
	Mem       *float64
	Disk      *float64
	Cmd       *string
	User      *string
	Shell     *bool
	Arguments *[]string
	Labels    *map[string]string
}

var updateJobSchema = schema{
	"type": "object",
	"properties": schema{
		"Schedule": schema{
			"type": []string{"object", "null"},
			"properties": schema{
				"Cron": schema{
					"type":   "string",
					"format": "cron",
				},
			},
			"required": []string{"Cron"},
		},
		"Container": schema{
			"type": []string{"object", "null"},
			"oneOf": []schema{
				schema{
					"properties": schema{
						"Docker": schema{
							"type": "object",
							"properties": schema{
								"Image": schema{
									"type":      []string{"string", "null"},
									"minLength": 1,
								},
								"ForcePullImage": schema{
									"type": []string{"boolean", "null"},
								},
							},
						},
					},
					"required": []string{},
				},
				schema{
					"properties": schema{
						"Mesos": schema{
							"type": "object",
							"properties": schema{
								"Image": schema{
									"type":      "string",
									"minLength": 1,
								},
							},
						},
					},
				},
			},
		},
		"CPUs": schema{
			"type":             []string{"number", "null"},
			"minimum":          0,
			"exclusiveMinimum": true,
		},
		"Disk": schema{
			"type":             []string{"number", "null"},
			"minimum":          0,
			"exclusiveMinimum": true,
		},
		"Mem": schema{
			"type":             []string{"number", "null"},
			"minimum":          0,
			"exclusiveMinimum": true,
		},
	},
}
