// Copyright (c) 2018, The GoKi Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package kit

import (
	"bytes"
	"fmt"
	"log"
	"reflect"
	"strings"

	"github.com/goki/ki/bitflag"
)

// design notes: for methods that return string, not passing error b/c you can
// easily check for null string, and registering errors in log for setter
// methods, returning error and also logging so it is safe to ignore err if
// you don't care

// Bit flags are setup just using the ordinal count iota, and the only diff is
// the methods which do 1 << flag when operating on them
// see bitflag package

// EnumRegistry is a map from an enum-style const int type name to a
// corresponding reflect.Type and conversion methods generated by (modified)
// stringer that convert to / from strings.
//
// Each such type must be explicitly registered by calling AddEnum
// in an expression that also initializes a new global variable
// that is then useful whenever you need to specify that type:
//
//     var KiT_MyEnum = kit.Enums.AddEnum(MyEnumN, bitFlag true/false,
//        TypeNameProps (or nil))
//
// where MyEnum is the name of the type, MyEnumN is the enum value
// representing the number of defined enums (always good practice to define
// this value, for ease of extension by others), and TypeNameProps is nil or a
// map[string]interface{} of properties, OR:
//
//     var KiT_MyEnum = kit.Enums.AddEnumAltLower(MyEnumN, bitFlag true/false,
//        TypeNameProps, "Prefix")
//
// which automatically registers alternative names as lower-case versions of
// const names with given prefix removed -- often what is used in e.g., json
// or xml kinds of formats.
//
// The resulting type name is registered using a *short* package-qualified
// version of the type name, with just the last directory name . Type.
// This is the usual name used in programming Go.  All properties are
// registered using this same short name.
//
// special properties:
//
// * "N": max value of enum defined -- number of enum entries (assuming
// ordinal, which is all that is currently supported here)
//
// * "BitFlag": true -- each value represents a bit in a set of bit flags, so
// the string rep of a value contains an or-list of names for each bit set,
// separated by "|".  Use the bitflag package to set and clear bits while
// keeping the definition of the flags as a standard ordinal integer value --
// much more flexible than pre-compiling the bitmasks.  Usually should be an
// int64 type.
//
// * "AltStrings": map[int64]string -- provides an alternative string mapping for
// the enum values
//
// Also recommend defining JSON I/O functions for each registered enum -- much
// safer to save enums as strings than using their raw numerical values, which
// can change over time:
//
//     func (ev TestFlags) MarshalJSON() ([]byte, error) { return kit.EnumMarshalJSON(ev) }
//     func (ev *TestFlags) UnmarshalJSON() ([]byte, error) { return kit.EnumUnmarshalJSON(ev) }
//
// And any value that will be used as a key in a map must define Text versions (which don't use quotes)
//
//     func (ev TestFlags) MarshalText() ([]byte, error) { return kit.EnumMarshalText(ev) }
//     func (ev *TestFlags) UnmarshalText() ([]byte, error) { return kit.EnumUnmarshalText(ev) }
//
type EnumRegistry struct {
	// Enums is a map from the *short* package-qualified name to reflect.Type
	Enums map[string]reflect.Type

	// Props contains properties that can be associated with each enum type
	// e.g., "BitFlag": true, "AltStrings" : map[int64]string, or other custom settings.
	// The key here is the short package-qualified name
	Props map[string]map[string]interface{}

	// Vals contains cached EnumValue representations of the enum values.
	// Used by Values method.
	Vals map[string][]EnumValue
}

// Enums is master registry of enum types -- can also create your own package-specific ones
var Enums EnumRegistry

// AddEnum adds a given type to the registry -- requires the N value to set N
// from and grab type info from -- if bitFlag then sets BitFlag property, and
// each value represents a bit in a set of bit flags, so the string rep of a
// value contains an or-list of names for each bit set, separated by | -- can
// also add additional properties -- they are copied so can be re-used across enums
func (tr *EnumRegistry) AddEnum(en interface{}, bitFlag bool, props map[string]interface{}) reflect.Type {
	if tr.Enums == nil {
		tr.Enums = make(map[string]reflect.Type)
		tr.Props = make(map[string]map[string]interface{})
		tr.Vals = make(map[string][]EnumValue)
	}

	// get the pointer-to version and elem so it is a settable type!
	typ := PtrType(reflect.TypeOf(en)).Elem()
	n := EnumIfaceToInt64(en)
	snm := ShortTypeName(typ)
	tr.Enums[snm] = typ
	if props != nil {
		// make a copy of props for enums -- often shared
		nwprops := make(map[string]interface{}, len(props))
		for key, val := range props {
			nwprops[key] = val
		}
		tr.Props[snm] = nwprops
	}
	tp := tr.Properties(snm)
	tp["N"] = n
	if bitFlag {
		tp := tr.Properties(snm)
		tp["BitFlag"] = true
		if n >= 64 {
			log.Printf("kit.AddEnum ERROR: enum: %v is a bitflag with more than 64 bits defined -- will likely not work: n: %v\n", snm, n)
			// } else { // if debug:
			// 	fmt.Printf("kit.AddEnum added bitflag enum: %v with n: %v\n", snm, n)
		}
	}
	// fmt.Printf("added enum: %v with n: %v\n", tn, n)
	return typ
}

// AddEnumAltLower adds a given type to the registry -- requires the N value
// to set N from and grab type info from -- automatically initializes
// AltStrings alternative string map based on the name with given prefix
// removed (e.g., a type name-based prefix) and lower-cased -- also requires
// the number of enums -- assumes starts at 0
func (tr *EnumRegistry) AddEnumAltLower(en interface{}, bitFlag bool, props map[string]interface{}, prefix string) reflect.Type {
	typ := tr.AddEnum(en, bitFlag, props)
	n := EnumIfaceToInt64(en)
	snm := ShortTypeName(typ)
	alts := make(map[int64]string)
	tp := tr.Properties(snm)
	for i := int64(0); i < n; i++ {
		str := EnumInt64ToString(i, typ)
		str = strings.ToLower(strings.TrimPrefix(str, prefix))
		alts[i] = str
	}
	tp["AltStrings"] = alts
	return typ
}

// Enum finds an enum type based on its *short* package-qualified type name
// returns nil if not found.
func (tr *EnumRegistry) Enum(name string) reflect.Type {
	return tr.Enums[name]
}

// TypeRegistered returns true if the given type is registered as an enum type.
func (tr *EnumRegistry) TypeRegistered(typ reflect.Type) bool {
	enumName := ShortTypeName(typ)
	_, ok := tr.Enums[enumName]
	return ok
}

// Props returns properties for this type based on short package-qualified name.
// Makes props map if not already made.
func (tr *EnumRegistry) Properties(enumName string) map[string]interface{} {
	tp, ok := tr.Props[enumName]
	if !ok {
		tp = make(map[string]interface{})
		tr.Props[enumName] = tp
	}
	return tp
}

// Prop safely finds an enum type property from short package-qualified name
// and property key.  Returns nil if not found.
func (tr *EnumRegistry) Prop(enumName, propKey string) interface{} {
	tp, ok := tr.Props[enumName]
	if !ok {
		// fmt.Printf("no props for enum type: %v\n", enumName)
		return nil
	}
	p, ok := tp[propKey]
	if !ok {
		// fmt.Printf("no props for key: %v\n", propKey)
		return nil
	}
	return p
}

// AltStrings returns optional alternative string map for enums -- e.g.,
// lower-case, without prefixes etc -- can put multiple such alt strings in
// the one string with your own separator, in a predefined order, if
// necessary, and just call strings.Split on those and get the one you want.
// Uses short package-qualified name. Returns nil if not set.
func (tr *EnumRegistry) AltStrings(enumName string) map[int64]string {
	ps := tr.Prop(enumName, "AltStrings")
	if ps == nil {
		return nil
	}
	m, ok := ps.(map[int64]string)
	if !ok {
		log.Printf("kit.EnumRegistry AltStrings error: AltStrings property must be a map[int64]string type, is not -- is instead: %T\n", m)
		return nil
	}
	return m
}

// NVals returns the number of defined enum values for given enum interface
func (tr *EnumRegistry) NVals(eval interface{}) int64 {
	typ := reflect.TypeOf(eval)
	n, _ := ToInt(tr.Prop(ShortTypeName(typ), "N"))
	return n
}

// IsBitFlag checks if this enum is for bit flags instead of mutually-exclusive int
// values -- checks BitFlag property -- if true string rep of a value contains
// an or-list of names for each bit set, separated by |
func (tr *EnumRegistry) IsBitFlag(typ reflect.Type) bool {
	b, _ := ToBool(tr.Prop(ShortTypeName(typ), "BitFlag"))
	return b
}

////////////////////////////////////////////////////////////////////////////////////////
//   To / From Int64 for generic interface{} and reflect.Value

// EnumIfaceToInt64 converts an enum interface{} into an int64 using reflect
// -- just use int64(eval) when you have the enum value in hand -- this is
// when you just have a generic interface{}
func EnumIfaceToInt64(eval interface{}) int64 {
	ev := NonPtrValue(reflect.ValueOf(eval))
	var ival int64
	reflect.ValueOf(&ival).Elem().Set(ev.Convert(reflect.TypeOf(ival)))
	return ival
}

// SetEnumIfaceFromInt64 sets enum interface{} value from int64 value -- must
// pass a pointer to the enum and also needs raw type of the enum as well --
// can't get it from the interface{} reliably
func SetEnumIfaceFromInt64(eval interface{}, ival int64, typ reflect.Type) error {
	if reflect.TypeOf(eval).Kind() != reflect.Ptr {
		err := fmt.Errorf("kit.SetEnumFromInt64: must pass a pointer to the enum: Type: %v, Kind: %v\n", reflect.TypeOf(eval).Name(), reflect.TypeOf(eval).Kind())
		log.Printf("%v", err)
		return err
	}
	reflect.ValueOf(eval).Elem().Set(reflect.ValueOf(ival).Convert(typ))
	return nil
}

// SetEnumValueFromInt64 sets enum value from int64 value, using a
// reflect.Value representation of the enum -- does more checking and can get
// type from value compared to Iface version
func SetEnumValueFromInt64(eval reflect.Value, ival int64) error {
	if eval.Kind() != reflect.Ptr {
		err := fmt.Errorf("kit.SetEnumValueFromInt64: must pass a pointer value to the enum: Type: %v, Kind: %v\n", eval.Type().String(), eval.Kind())
		log.Printf("%v", err)
		return err
	}
	npt := NonPtrType(eval.Type())
	eval.Elem().Set(reflect.ValueOf(ival).Convert(npt))
	return nil
}

// EnumIfaceFromInt64 returns an interface{} value which is an enum value of
// given type (not a pointer to it), set to given integer value
func EnumIfaceFromInt64(ival int64, typ reflect.Type) interface{} {
	evn := reflect.New(typ)
	SetEnumValueFromInt64(evn, ival)
	return evn.Elem().Interface()
}

////////////////////////////////////////////////////////////////////////////////////////
//   To / From String for generic interface{} and reflect.Value

// EnumIfaceToString converts an enum interface{} value to its corresponding
// string value, using fmt.Stringer interface directly -- same effect as
// calling fmt.Sprintf("%v") but this is slightly faster
func EnumIfaceToString(eval interface{}) string {
	strer, ok := eval.(fmt.Stringer) // will fail if not impl
	if !ok {
		log.Printf("kit.EnumIfaceToString: fmt.Stringer interface not supported by type %v\n", reflect.TypeOf(eval).Name())
		return ""
	}
	return strer.String()
}

// EnumInt64ToString first converts an int64 to enum of given type, and then
// converts that to a string value
func EnumInt64ToString(ival int64, typ reflect.Type) string {
	ev := EnumIfaceFromInt64(ival, typ)
	return EnumIfaceToString(ev)
}

// EnumIfaceFromString returns an interface{} value which is an enum value of
// given type (not a pointer to it), set to given string value -- requires
// reflect type of enum
func EnumIfaceFromString(str string, typ reflect.Type) interface{} {
	evn := reflect.New(typ)
	SetEnumValueFromString(evn, str)
	return evn.Elem().Interface()
}

// EnumIfaceToAltString converts an enum interface{} value to its
// corresponding alternative string value from the enum registry
func (tr *EnumRegistry) EnumIfaceToAltString(eval interface{}) string {
	if reflect.TypeOf(eval).Kind() == reflect.Ptr {
		eval = reflect.ValueOf(eval).Elem() // deref the pointer
	}
	et := reflect.TypeOf(eval)
	tn := ShortTypeName(et)
	alts := tr.AltStrings(tn)
	if alts == nil {
		log.Printf("kit.EnumToAltString: no alternative string map for type %v\n", tn)
		return ""
	}
	// convert to int64 for lookup
	ival := EnumIfaceToInt64(eval)
	return alts[ival]
}

// EnumInt64ToAltString converts an int64 value to the enum of given type, and
// then into corresponding alternative string value
func (tr *EnumRegistry) EnumInt64ToAltString(ival int64, typnm string) string {
	alts := tr.AltStrings(typnm)
	if alts == nil {
		log.Printf("kit.EnumInt64ToAltString: no alternative string map for type %v\n", typnm)
		return ""
	}
	return alts[ival]
}

// SetEnumValueFromString sets enum value from string using reflect.Value
// IMPORTANT: requires the modified stringer go generate utility
// that generates a StringToTypeName method
func SetEnumValueFromString(eval reflect.Value, str string) error {
	etp := eval.Type()
	if etp.Kind() != reflect.Ptr {
		err := fmt.Errorf("kit.SetEnumValueFromString -- you must pass a pointer enum, not type: %v kind %v\n", etp, etp.Kind())
		// log.Printf("%v", err)
		return err
	}
	et := etp.Elem()
	methnm := "FromString"
	meth := eval.MethodByName(methnm)
	if ValueIsZero(meth) || meth.IsNil() {
		err := fmt.Errorf("kit.SetEnumValueFromString: stringer-generated FromString() method not found: %v for type: %v %T\n", methnm, et.Name(), eval.Interface())
		log.Printf("%v", err)
		return err
	}
	sv := reflect.ValueOf(str)
	args := make([]reflect.Value, 1)
	args[0] = sv
	meth.Call(args)
	// fmt.Printf("return from FromString method: %v\n", rv[0].Interface())
	return nil
}

// SetEnumIfaceFromString sets enum value from string -- must pass a *pointer*
// to the enum item. IMPORTANT: requires the modified stringer go generate
// utility that generates a StringToTypeName method
func SetEnumIfaceFromString(eptr interface{}, str string) error {
	return SetEnumValueFromString(reflect.ValueOf(eptr), str)
}

// SetEnumValueFromAltString sets value from alternative string using a
// reflect.Value -- must pass a *pointer* value to the enum item.
func (tr *EnumRegistry) SetEnumValueFromAltString(eval reflect.Value, str string) error {
	etp := eval.Type()
	if etp.Kind() != reflect.Ptr {
		err := fmt.Errorf("kit.SetEnumValueFromString -- you must pass a pointer enum, not type: %v kind %v\n", etp, etp.Kind())
		log.Printf("%v", err)
		return err
	}
	et := etp.Elem()
	tn := ShortTypeName(et)
	alts := tr.AltStrings(tn)
	if alts == nil {
		err := fmt.Errorf("kit.SetEnumValueFromAltString: no alternative string map for type %v\n", tn)
		// log.Printf("%v", err)
		return err
	}
	for i, v := range alts {
		if v == str {
			return SetEnumValueFromInt64(eval, int64(i))
		}
	}
	err := fmt.Errorf("kit.SetEnumValueFromAltString: string: %v not found in alt list of strings for type%v\n", str, tn)
	// log.Printf("%v", err)
	return err
}

// SetEnumIfaceFromAltString sets from alternative string list using an interface{}
// to the enum -- must pass a *pointer* to the enum item.
func (tr *EnumRegistry) SetEnumIfaceFromAltString(eptr interface{}, str string) error {
	return tr.SetEnumValueFromAltString(reflect.ValueOf(eptr), str)
}

// SetEnumValueFromStringAltFirst first attempts to set an enum from an
// alternative string, and if that fails, then it tries to set from the
// regular string representation func (tr *EnumRegistry)
func (tr *EnumRegistry) SetEnumValueFromStringAltFirst(eval reflect.Value, str string) error {
	err := tr.SetEnumValueFromAltString(eval, str)
	if err != nil {
		return SetEnumValueFromString(eval, str)
	}
	return err
}

// SetEnumIfaceFromStringAltFirst first attempts to set an enum from an
// alternative string, and if that fails, then it tries to set from the
// regular string representation func (tr *EnumRegistry)
func (tr *EnumRegistry) SetEnumIfaceFromStringAltFirst(eptr interface{}, str string) error {
	err := tr.SetEnumIfaceFromAltString(eptr, str)
	if err != nil {
		return SetEnumIfaceFromString(eptr, str)
	}
	return err
}

///////////////////////////////////////////////////////////////////////////////
//  BitFlags

// BitFlagsToString converts an int64 of bit flags into a string
// representation of the bits that are set -- en is the number of defined
// bits, and also provides the type name for looking up strings
func BitFlagsToString(bflg int64, en interface{}) string {
	et := PtrType(reflect.TypeOf(en)).Elem()
	n := int(EnumIfaceToInt64(en))
	str := ""
	for i := 0; i < n; i++ {
		if bitflag.Has(bflg, i) {
			evs := EnumInt64ToString(int64(i), et)
			if str == "" {
				str = evs
			} else {
				str += "|" + evs
			}
		}
	}
	return str
}

// BitFlagsFromString sets an int64 of bit flags from a string representation
// of the bits that are set -- en is the number of defined bits, and also
// provides the type name for looking up strings
func BitFlagsFromString(bflg *int64, str string, en interface{}) error {
	et := PtrType(reflect.TypeOf(en)).Elem()
	n := int(EnumIfaceToInt64(en))
	return BitFlagsTypeFromString(bflg, str, et, n)
}

// BitFlagsTypeFromString sets an int64 of bit flags from a string representation
// of the bits that are set -- gets enum type and n of defined elements directly
func BitFlagsTypeFromString(bflg *int64, str string, et reflect.Type, n int) error {
	flgs := strings.Split(str, "|")
	evv := reflect.New(et)
	var err error
	for _, flg := range flgs {
		err = SetEnumValueFromString(evv, flg)
		if err == nil {
			evi := EnumIfaceToInt64(evv.Interface())
			bitflag.Set(bflg, int(evi))
		}
	}
	return err
}

// BitFlagsFromStringAltFirst sets an int64 of bit flags from a string
// representation of the bits that are set, using alt-strings first -- gets
// enum type and n of defined elements directly
func (tr *EnumRegistry) BitFlagsFromStringAltFirst(bflg *int64, str string, et reflect.Type, n int) error {
	flgs := strings.Split(str, "|")
	evv := reflect.New(et)
	var err error
	for _, flg := range flgs {
		err = tr.SetEnumValueFromStringAltFirst(evv, flg)
		if err == nil {
			evi := EnumIfaceToInt64(evv.Interface())
			bitflag.Set(bflg, int(evi))
		}
	}
	return err
}

// SetAnyEnumValueFromString looks up enum type on registry, and if it is
// registered as a bitflag, sets bits from string, otherwise tries to set from
// alt strings if those exist, and finally tries direct set from string --
// must pass a *pointer* value to the enum item.
func (tr *EnumRegistry) SetAnyEnumValueFromString(eval reflect.Value, str string) error {
	etp := eval.Type()
	if etp.Kind() != reflect.Ptr {
		err := fmt.Errorf("kit.SetAnyEnumValueFromString -- you must pass a pointer enum, not type: %v kind %v\n", etp, etp.Kind())
		log.Printf("%v", err)
		return err
	}
	et := etp.Elem()
	if tr.IsBitFlag(et) {
		var bf int64
		err := tr.BitFlagsFromStringAltFirst(&bf, str, et, int(tr.NVals(eval.Interface())))
		if err != nil {
			return err
		}
		return SetEnumValueFromInt64(eval, bf)
	} else {
		return tr.SetEnumValueFromStringAltFirst(eval, str)
	}
}

// SetAnyEnumIfaceFromString looks up enum type on registry, and if it is
// registered as a bitflag, sets bits from string, otherwise tries to set from
// alt strings if those exist, and finally tries direct set from string --
// must pass a *pointer* value to the enum item.
func (tr *EnumRegistry) SetAnyEnumIfaceFromString(eptr interface{}, str string) error {
	return tr.SetAnyEnumValueFromString(reflect.ValueOf(eptr), str)
}

///////////////////////////////////////////////////////////////////////////////
//  EnumValue

// EnumValue represents enum values, in common int64 terms, e.g., for GUI
type EnumValue struct {
	Name  string       `desc:"name for this value"`
	Value int64        `desc:"integer value"`
	Type  reflect.Type `desc:"the enum type that this value belongs to"`
}

// Set sets the values of the EnumValue struct
func (ev *EnumValue) Set(name string, val int64, typ reflect.Type) {
	ev.Name = name
	ev.Value = val
	ev.Type = typ
}

// String satisfies fmt.Stringer and provides a string representation of enum: just the name
func (ev EnumValue) String() string {
	return ev.Name
}

// Values returns an EnumValue slice for all the values of an enum type -- if
// alt is true and alt names exist, then those are used
func (tr *EnumRegistry) Values(enumName string, alt bool) []EnumValue {
	vals, ok := tr.Vals[enumName]
	if ok {
		return vals
	}
	alts := tr.AltStrings(enumName)
	et := tr.Enums[enumName]
	n := tr.Prop(enumName, "N").(int64)
	vals = make([]EnumValue, n)
	for i := int64(0); i < n; i++ {
		str := EnumInt64ToString(i, et) // todo: what happens when no string for given values?
		if alt && alts != nil {
			str = alts[i]
		}
		vals[i].Set(str, i, et)
	}
	tr.Vals[enumName] = vals
	return vals
}

// TypeValues returns an EnumValue slice for all the values of an enum type --
// if alt is true and alt names exist, then those are used
func (tr *EnumRegistry) TypeValues(et reflect.Type, alt bool) []EnumValue {
	return tr.Values(ShortTypeName(et), alt)
}

// AllTagged returns a list of all registered enum types that include a given
// property key value -- does not check for the value of that value -- just
// its existence
func (tr *EnumRegistry) AllTagged(key string) []reflect.Type {
	tl := make([]reflect.Type, 0)
	for _, typ := range tr.Enums {
		tp := tr.Prop(ShortTypeName(typ), key)
		if tp == nil {
			continue
		}
		tl = append(tl, typ)
	}
	return tl
}

///////////////////////////////////////////////////////////////////////////////
//  JSON, Text Marshal

func EnumMarshalJSON(eval interface{}) ([]byte, error) {
	et := reflect.TypeOf(eval)
	b := make([]byte, 0, 50)
	b = append(b, []byte("\"")...)
	if Enums.IsBitFlag(et) {
		b = append(b, []byte(BitFlagsToString(EnumIfaceToInt64(eval), eval))...)
	} else {
		b = append(b, []byte(EnumIfaceToString(eval))...)
	}
	b = append(b, []byte("\"")...)
	return b, nil
}

func EnumUnmarshalJSON(eval interface{}, b []byte) error {
	et := reflect.TypeOf(eval)
	noq := string(bytes.Trim(b, "\""))
	if Enums.IsBitFlag(et) {
		bf := int64(0)
		err := BitFlagsTypeFromString(&bf, noq, et, int(Enums.NVals(eval)))
		if err == nil {
			return SetEnumIfaceFromInt64(eval, bf, et)
		}
		return err
	} else {
		return SetEnumIfaceFromString(eval, noq)
	}
}

func EnumMarshalText(eval interface{}) ([]byte, error) {
	et := reflect.TypeOf(eval)
	b := make([]byte, 0, 50)
	if Enums.IsBitFlag(et) {
		b = append(b, []byte(BitFlagsToString(EnumIfaceToInt64(eval), eval))...)
	} else {
		b = append(b, []byte(EnumIfaceToString(eval))...)
	}
	return b, nil
}

func EnumUnmarshalText(eval interface{}, b []byte) error {
	et := reflect.TypeOf(eval)
	noq := string(b)
	if Enums.IsBitFlag(et) {
		bf := int64(0)
		err := BitFlagsTypeFromString(&bf, noq, et, int(Enums.NVals(eval)))
		if err == nil {
			return SetEnumIfaceFromInt64(eval, bf, et)
		}
		return err
	} else {
		return SetEnumIfaceFromString(eval, noq)
	}
}

/////////////////////////////////////////////////////////////
// Following is for testing..

// testing
type TestFlags int32

const (
	TestFlagsNil TestFlags = iota
	TestFlag1
	TestFlag2
	TestFlagsN
)

//go:generate stringer -type=TestFlags

var KiT_TestFlags = Enums.AddEnumAltLower(TestFlagsN, false, nil, "Test")

func (ev TestFlags) MarshalJSON() ([]byte, error)  { return EnumMarshalJSON(ev) }
func (ev *TestFlags) UnmarshalJSON(b []byte) error { return EnumUnmarshalJSON(ev, b) }
