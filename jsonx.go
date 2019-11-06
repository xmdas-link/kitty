package kitty

import (
	"time"
	"unsafe"

	"github.com/iancoleman/strcase"
	jsoniter "github.com/json-iterator/go"
	"github.com/modern-go/reflect2"
)

// omitEncoder ...
type omitEncoder struct {
	ValEncoder jsoniter.ValEncoder
}

func (encoder *omitEncoder) Encode(ptr unsafe.Pointer, stream *jsoniter.Stream) {
}
func (encoder *omitEncoder) IsEmpty(ptr unsafe.Pointer) bool {
	return true
}
func (encoder *omitEncoder) IsEmbeddedPtrNil(ptr unsafe.Pointer) bool {
	return true
}

type filterFieldsExtension struct {
	jsoniter.DummyExtension
	Fields []string
	Name   string //model name
}

func (extension *filterFieldsExtension) UpdateStructDescriptor(structDescriptor *jsoniter.StructDescriptor) {
	for _, binding := range structDescriptor.Fields {
		if jsonTag := binding.Field.Tag().Get("json"); len(jsonTag) > 0 {
			if jsonTag != "omitempty" {
				continue
			}
		}
		binding.ToNames = []string{strcase.ToSnake(binding.Field.Name())}
	}
	if len(extension.Name) > 0 {
		structType := structDescriptor.Type.(*reflect2.UnsafeStructType)
		name := structType.Name()
		if name != extension.Name {
			return
		}
	}

	if len(extension.Fields) == 0 {
		return
	}

	for _, binding := range structDescriptor.Fields {
		defaultEncoder := binding.Encoder
		binding.Encoder = &omitEncoder{ValEncoder: binding.Encoder}

		name := strcase.ToSnake(binding.Field.Name())
		for _, v := range extension.Fields {
			if name == v {
				binding.Encoder = defaultEncoder
				break
			}
		}
	}
}

//TimeAsString format time.Time as string
type TimeAsString struct {
	precision time.Duration
}

//Decode string to time.Time
func (codec *TimeAsString) Decode(ptr unsafe.Pointer, iter *jsoniter.Iterator) {
	//	nanoseconds := iter.ReadInt64() * codec.precision.Nanoseconds()
	stamp, _ := time.ParseInLocation("2006-01-02 15:04:05", iter.ReadString(), time.Local)
	*((*time.Time)(ptr)) = stamp
}

//IsEmpty ..
func (codec *TimeAsString) IsEmpty(ptr unsafe.Pointer) bool {
	ts := *((*time.Time)(ptr))
	return ts.UnixNano() == 0
}

//Encode time.Time to string
func (codec *TimeAsString) Encode(ptr unsafe.Pointer, stream *jsoniter.Stream) {
	ts := *((*time.Time)(ptr))
	stream.WriteString(ts.Format("2006-01-02 15:04:05"))
	//	stream.WriteInt64(ts.UnixNano() / codec.precision.Nanoseconds())
}
