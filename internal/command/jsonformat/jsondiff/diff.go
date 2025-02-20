package jsondiff

import (
	"reflect"

	"github.com/hashicorp/terraform/internal/command/jsonformat/collections"
	"github.com/hashicorp/terraform/internal/command/jsonformat/computed"

	"github.com/zclconf/go-cty/cty"

	"github.com/hashicorp/terraform/internal/plans"
)

type TransformPrimitiveJson func(before, after interface{}, ctype cty.Type, action plans.Action) computed.Diff
type TransformObjectJson func(map[string]computed.Diff, plans.Action) computed.Diff
type TransformArrayJson func([]computed.Diff, plans.Action) computed.Diff
type TransformTypeChangeJson func(before, after computed.Diff, action plans.Action) computed.Diff

// JsonOpts defines the external callback functions that callers should
// implement to process the supplied diffs.
type JsonOpts struct {
	Primitive  TransformPrimitiveJson
	Object     TransformObjectJson
	Array      TransformArrayJson
	TypeChange TransformTypeChangeJson
}

// Transform accepts a generic before and after value that is assumed to be JSON
// formatted and transforms it into a computed.Diff, using the callbacks
// supplied in the JsonOpts class.
func (opts JsonOpts) Transform(before, after interface{}) computed.Diff {
	beforeType := GetType(before)
	afterType := GetType(after)

	if beforeType == afterType || (beforeType == Null || afterType == Null) {
		targetType := beforeType
		if targetType == Null {
			targetType = afterType
		}
		return opts.processUpdate(before, after, targetType)
	}

	b := opts.processUpdate(before, nil, beforeType)
	a := opts.processUpdate(nil, after, afterType)
	return opts.TypeChange(b, a, plans.Update)
}

func (opts JsonOpts) processUpdate(before, after interface{}, jtype Type) computed.Diff {
	switch jtype {
	case Null:
		return opts.processPrimitive(before, after, cty.NilType)
	case Bool:
		return opts.processPrimitive(before, after, cty.Bool)
	case String:
		return opts.processPrimitive(before, after, cty.String)
	case Number:
		return opts.processPrimitive(before, after, cty.Number)
	case Object:
		var b, a map[string]interface{}

		if before != nil {
			b = before.(map[string]interface{})
		}

		if after != nil {
			a = after.(map[string]interface{})
		}

		return opts.processObject(b, a)
	case Array:
		var b, a []interface{}

		if before != nil {
			b = before.([]interface{})
		}

		if after != nil {
			a = after.([]interface{})
		}

		return opts.processArray(b, a)
	default:
		panic("unrecognized json type: " + jtype)
	}
}

func (opts JsonOpts) processPrimitive(before, after interface{}, ctype cty.Type) computed.Diff {
	var action plans.Action
	switch {
	case before == nil && after != nil:
		action = plans.Create
	case before != nil && after == nil:
		action = plans.Delete
	case reflect.DeepEqual(before, after):
		action = plans.NoOp
	default:
		action = plans.Update
	}

	return opts.Primitive(before, after, ctype, action)
}

func (opts JsonOpts) processArray(before, after []interface{}) computed.Diff {
	processIndices := func(beforeIx, afterIx int) computed.Diff {
		var b, a interface{}

		if beforeIx >= 0 && beforeIx < len(before) {
			b = before[beforeIx]
		}

		if afterIx >= 0 && afterIx < len(after) {
			a = after[afterIx]
		}

		return opts.Transform(b, a)
	}

	isObjType := func(value interface{}) bool {
		return GetType(value) == Object
	}

	return opts.Array(collections.TransformSlice(before, after, processIndices, isObjType))
}

func (opts JsonOpts) processObject(before, after map[string]interface{}) computed.Diff {
	return opts.Object(collections.TransformMap(before, after, func(key string) computed.Diff {
		return opts.Transform(before[key], after[key])
	}))
}
