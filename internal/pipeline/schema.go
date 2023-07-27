package pipeline

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/turbot/flowpipe/pipeparser/schema"
)

var PipelineBlockSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{
			Name:     schema.AttributeTypeDescription,
			Required: false,
		},
	},
	Blocks: []hcl.BlockHeaderSchema{
		{
			Type:       schema.BlockTypeParam,
			LabelNames: []string{schema.LabelName},
		},
		{
			Type:       schema.BlockTypePipeline,
			LabelNames: []string{schema.LabelName},
		},
		{
			Type:       schema.BlockTypePipelineStep,
			LabelNames: []string{schema.LabelType, schema.LabelName},
		},
		{
			Type:       schema.BlockTypePipelineOutput,
			LabelNames: []string{schema.LabelName},
		},
	},
}

var PipelineOutputBlockSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{
			Name: schema.AttributeTypeTitle,
		},
		{
			Name: schema.AttributeTypeDescription,
		},
		{
			Name: schema.AttributeTypeForEach,
		},
		{
			Name: schema.AttributeTypeDependsOn,
		},
		{
			Name:     schema.AttributeTypeValue,
			Required: true,
		},
		{
			Name: schema.AttributeTypeSensitive,
		},
	},
}

var PipelineStepHttpBlockSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{
			Name: schema.AttributeTypeTitle,
		},
		{
			Name: schema.AttributeTypeDescription,
		},
		{
			Name: schema.AttributeTypeForEach,
		},
		{
			Name: schema.AttributeTypeDependsOn,
		},
		{
			Name: schema.AttributeTypeIf,
		},
		{
			Name:     schema.AttributeTypeUrl,
			Required: true,
		},
		{
			Name: schema.AttributeTypeMethod,
		},
		{
			Name: schema.AttributeTypeRequestTimeoutMs,
		},
		{
			Name: schema.AttributeTypeInsecure,
		},
		{
			Name: schema.AttributeTypeRequestBody,
		},
		{
			Name: schema.AttributeTypeRequestHeaders,
		},
	},
	Blocks: []hcl.BlockHeaderSchema{
		{
			Type: schema.BlockTypeError,
		},
	},
}

var PipelineStepSleepBlockSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{
			Name: schema.AttributeTypeTitle,
		},
		{
			Name: schema.AttributeTypeDescription,
		},
		{
			Name: schema.AttributeTypeForEach,
		},
		{
			Name: schema.AttributeTypeDependsOn,
		},
		{
			Name: schema.AttributeTypeIf,
		},
		{
			Name:     schema.AttributeTypeDuration,
			Required: true,
		},
	},
}

var PipelineStepEmailBlockSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{
			Name: schema.AttributeTypeTitle,
		},
		{
			Name: schema.AttributeTypeDescription,
		},
		{
			Name: schema.AttributeTypeForEach,
		},
		{
			Name: schema.AttributeTypeDependsOn,
		},
		{
			Name: schema.AttributeTypeIf,
		},
		{
			Name:     schema.AttributeTypeTo,
			Required: true,
		},
	},
}

var PipelineStepQueryBlockSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{
			Name: schema.AttributeTypeTitle,
		},
		{
			Name: schema.AttributeTypeDescription,
		},
		{
			Name: schema.AttributeTypeForEach,
		},
		{
			Name: schema.AttributeTypeDependsOn,
		},
		{
			Name: schema.AttributeTypeIf,
		},
		{
			Name: schema.AttributeTypeSql,
		},
		{
			Name: schema.AttributeTypeConnectionString,
		},
		{
			Name: schema.AttributeTypeArgs,
		},
	},
}

var PipelineStepEchoBlockSchema = &hcl.BodySchema{
	Attributes: []hcl.AttributeSchema{
		{
			Name: schema.AttributeTypeTitle,
		},
		{
			Name: schema.AttributeTypeDescription,
		},
		{
			Name: schema.AttributeTypeForEach,
		},
		{
			Name: schema.AttributeTypeDependsOn,
		},
		{
			Name: schema.AttributeTypeIf,
		},
		{
			Name: schema.AttributeTypeText,
		},

		{
			Name: "list_text",
		},
		{
			Name: "for_each",
		},
	},
}
