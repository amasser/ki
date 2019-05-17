// Code generated by "stringer -type=Flags"; DO NOT EDIT.

package ki

import (
	"errors"
	"strconv"
)

var _ = errors.New("dummy error")

const _Flags_name = "IsFieldHasKiFieldsHasNoKiFieldsUpdatingOnlySelfUpdateNodeAddedNodeCopiedNodeMovedNodeDeletedNodeDestroyedChildAddedChildMovedChildDeletedChildrenDeletedFieldUpdatedPropUpdatedFlagsN"

var _Flags_index = [...]uint8{0, 7, 18, 31, 39, 53, 62, 72, 81, 92, 105, 115, 125, 137, 152, 164, 175, 181}

func (i Flags) String() string {
	if i < 0 || i >= Flags(len(_Flags_index)-1) {
		return "Flags(" + strconv.FormatInt(int64(i), 10) + ")"
	}
	return _Flags_name[_Flags_index[i]:_Flags_index[i+1]]
}

func (i *Flags) FromString(s string) error {
	for j := 0; j < len(_Flags_index)-1; j++ {
		if s == _Flags_name[_Flags_index[j]:_Flags_index[j+1]] {
			*i = Flags(j)
			return nil
		}
	}
	return errors.New("String: " + s + " is not a valid option for type: Flags")
}
