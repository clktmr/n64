package fixed

type Int8 int8
type Point8 = Point[Int8]
type Rectangle8 = Rectangle[Int8]

func (x Int8) Mul(y Int8) Int8 { return x * y }
func (x Int8) Div(y Int8) Int8 { return x / y }

type UInt8 uint8
type PointU8 = Point[UInt8]
type RectangleU8 = Rectangle[UInt8]

func (x UInt8) Mul(y UInt8) UInt8 { return x * y }
func (x UInt8) Div(y UInt8) UInt8 { return x / y }

type Int16 int16
type Point16 = Point[Int16]
type Rectangle16 = Rectangle[Int16]

func (x Int16) Mul(y Int16) Int16 { return x * y }
func (x Int16) Div(y Int16) Int16 { return x / y }

type UInt16 uint16
type PointU16 = Point[UInt16]
type RectangleU16 = Rectangle[UInt16]

func (x UInt16) Mul(y UInt16) UInt16 { return x * y }
func (x UInt16) Div(y UInt16) UInt16 { return x / y }

type Int32 int32
type Point32 = Point[Int32]
type Rectangle32 = Rectangle[Int32]

func (x Int32) Mul(y Int32) Int32 { return x * y }
func (x Int32) Div(y Int32) Int32 { return x / y }

type UInt32 uint32
type PointU32 = Point[UInt32]
type RectangleU32 = Rectangle[UInt32]

func (x UInt32) Mul(y UInt32) UInt32 { return x * y }
func (x UInt32) Div(y UInt32) UInt32 { return x / y }

type Int64 int64
type Point64 = Point[Int64]
type Rectangle64 = Rectangle[Int64]

func (x Int64) Mul(y Int64) Int64 { return x * y }
func (x Int64) Div(y Int64) Int64 { return x / y }

type UInt64 uint64
type PointU64 = Point[UInt64]
type RectangleU64 = Rectangle[UInt64]

func (x UInt64) Mul(y UInt64) UInt64 { return x * y }
func (x UInt64) Div(y UInt64) UInt64 { return x / y }
