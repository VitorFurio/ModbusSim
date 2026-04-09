package register

type DataType string

const (
	TypeUint16  DataType = "uint16"
	TypeInt16   DataType = "int16"
	TypeUint32  DataType = "uint32"
	TypeInt32   DataType = "int32"
	TypeFloat32 DataType = "float32"
	TypeBool    DataType = "bool"
)

// WordCount returns how many 16-bit Modbus words this type occupies.
func (d DataType) WordCount() uint16 {
	switch d {
	case TypeUint32, TypeInt32, TypeFloat32:
		return 2
	default:
		return 1
	}
}

type SignalKind string

const (
	SignalConstant      SignalKind = "constant"
	SignalSine          SignalKind = "sine"
	SignalRamp          SignalKind = "ramp"
	SignalCounter       SignalKind = "counter"
	SignalCounterRandom SignalKind = "counter_random"
	SignalRandomWalk    SignalKind = "random_walk"
	SignalStep          SignalKind = "step"
)

type Signal struct {
	Kind        SignalKind `yaml:"kind" json:"kind"`
	Value       float64   `yaml:"value,omitempty" json:"value,omitempty"`
	Amplitude   float64   `yaml:"amplitude,omitempty" json:"amplitude,omitempty"`
	Period      float64   `yaml:"period,omitempty" json:"period,omitempty"`
	Offset      float64   `yaml:"offset,omitempty" json:"offset,omitempty"`
	Rate        float64   `yaml:"rate,omitempty" json:"rate,omitempty"`
	StepMin     float64   `yaml:"step_min,omitempty" json:"step_min,omitempty"`
	StepMax     float64   `yaml:"step_max,omitempty" json:"step_max,omitempty"`
	IntervalMs  int       `yaml:"interval_ms,omitempty" json:"interval_ms,omitempty"`
	StepMaxWalk float64   `yaml:"step_max_walk,omitempty" json:"step_max_walk,omitempty"`
	Low         float64   `yaml:"low,omitempty" json:"low,omitempty"`
	High        float64   `yaml:"high,omitempty" json:"high,omitempty"`
	Min         float64   `yaml:"min,omitempty" json:"min,omitempty"`
	Max         float64   `yaml:"max,omitempty" json:"max,omitempty"`
}

type Register struct {
	ID          string   `yaml:"id" json:"id"`
	Name        string   `yaml:"name" json:"name"`
	Description string   `yaml:"description,omitempty" json:"description,omitempty"`
	Address     uint16   `yaml:"address" json:"address"`
	DataType    DataType `yaml:"data_type" json:"data_type"`
	Unit        string   `yaml:"unit,omitempty" json:"unit,omitempty"`
	Signal      Signal   `yaml:"signal" json:"signal"`
	Value       float64  `yaml:"-" json:"value"`
	UpdatedAt   int64    `yaml:"-" json:"updated_at"`
}

func (r *Register) WordAddresses() []uint16 {
	count := r.DataType.WordCount()
	addrs := make([]uint16, count)
	for i := range addrs {
		addrs[i] = r.Address + uint16(i)
	}
	return addrs
}
