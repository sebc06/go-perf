// Code generated by "bitstringer -type=SampleFormat"; DO NOT EDIT

package perffile

import "strconv"

func (i SampleFormat) String() string {
	if i == 0 {
		return "0"
	}
	s := ""
	if i&SampleFormatAddr != 0 {
		s += "Addr|"
	}
	if i&SampleFormatBranchStack != 0 {
		s += "BranchStack|"
	}
	if i&SampleFormatCPU != 0 {
		s += "CPU|"
	}
	if i&SampleFormatCallchain != 0 {
		s += "Callchain|"
	}
	if i&SampleFormatDataSrc != 0 {
		s += "DataSrc|"
	}
	if i&SampleFormatID != 0 {
		s += "ID|"
	}
	if i&SampleFormatIP != 0 {
		s += "IP|"
	}
	if i&SampleFormatIdentifier != 0 {
		s += "Identifier|"
	}
	if i&SampleFormatPeriod != 0 {
		s += "Period|"
	}
	if i&SampleFormatRaw != 0 {
		s += "Raw|"
	}
	if i&SampleFormatRead != 0 {
		s += "Read|"
	}
	if i&SampleFormatRegsIntr != 0 {
		s += "RegsIntr|"
	}
	if i&SampleFormatRegsUser != 0 {
		s += "RegsUser|"
	}
	if i&SampleFormatStackUser != 0 {
		s += "StackUser|"
	}
	if i&SampleFormatStreamID != 0 {
		s += "StreamID|"
	}
	if i&SampleFormatTID != 0 {
		s += "TID|"
	}
	if i&SampleFormatTime != 0 {
		s += "Time|"
	}
	if i&SampleFormatTransaction != 0 {
		s += "Transaction|"
	}
	if i&SampleFormatWeight != 0 {
		s += "Weight|"
	}
	i &^= 524287
	if i == 0 {
		return s[:len(s)-1]
	}
	return s + "0x" + strconv.FormatUint(uint64(i), 16)
}