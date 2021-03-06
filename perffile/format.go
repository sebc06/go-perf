// Copyright 2015 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package perffile

import (
	"fmt"
	"io"
)

const numFeatureBits = 256

// perf_file_header from tools/perf/util/header.h
type fileHeader struct {
	Magic    [8]byte
	Size     uint64      // Size of fileHeader on disk
	AttrSize uint64      // Size of fileAttr on disk
	Attrs    fileSection // Array of fileAttr
	Data     fileSection // Alternating recordHeader and record
	_        fileSection // event_types; ignored in v2

	Features [numFeatureBits / 64]uint64 // Bitmap of feature
}

func (h *fileHeader) hasFeature(f feature) bool {
	return h.Features[f/64]&(1<<(uint(f)%64)) != 0
}

// perf_file_section from tools/perf/util/header.h
type fileSection struct {
	Offset, Size uint64
}

func (s fileSection) sectionReader(r io.ReaderAt) *io.SectionReader {
	return io.NewSectionReader(r, int64(s.Offset), int64(s.Size))
}

func (s fileSection) data(r io.ReaderAt) ([]byte, error) {
	out := make([]byte, s.Size)
	n, err := r.ReadAt(out, int64(s.Offset))
	if n == len(out) {
		return out, nil
	}
	return nil, err
}

// HEADER_* enum from tools/perf/util/header.h
type feature int

const (
	featureReserved feature = iota // always cleared
	featureTracingData
	featureBuildID

	featureHostname
	featureOSRelease
	featureVersion
	featureArch
	featureNrCpus
	featureCPUDesc
	featureCPUID
	featureTotalMem
	featureCmdline
	featureEventDesc
	featureCPUTopology
	featureNUMATopology
	featureBranchStack
	featurePMUMappings
	featureGroupDesc
)

// perf_file_attr from tools/perf/util/header.c
type fileAttr struct {
	Attr EventAttr
	IDs  fileSection // array of attrID, one per core/thread
}

// eventAttrV0 is on-disk version 0 of the perf_event_attr structure.
// Later versions extended this with additional fields, but the header
// is always the same.
type eventAttrV0 struct {
	Type                    EventType
	Size                    uint32
	Config                  uint64
	SamplePeriodOrFreq      uint64
	SampleFormat            SampleFormat
	ReadFormat              ReadFormat
	Flags                   EventFlags
	WakeupEventsOrWatermark uint32
	BPType                  uint32
	BPAddrOrConfig1         uint64
}

// eventAttrVN is the on-disk latest version of the perf_event_attr
// structure (currently version 5).
type eventAttrVN struct {
	eventAttrV0

	// ABI v1
	BPLenOrConfig2 uint64

	// ABI v2
	BranchSampleType uint64

	// ABI v3
	SampleRegsUser  uint64
	SampleStackUser uint32
	ClockID         int32

	// ABI v4
	SampleRegsIntr uint64

	// ABI v5
	AuxWatermark uint32
	Pad          uint32 // Align to uint64
}

// TODO: Make public
type attrID uint64

// Event describes a specific performance monitoring event.
//
// Events are quite general. They can be hardware events such as
// cycles or cache misses. They can be kernel software events such as
// page faults. They can be user or kernel trace-points, or many other
// things. All events happen at some instant and can be counted.
type Event interface {
	// Generic returns the generic representation of this Event.
	Generic() EventGeneric
}

// An EventType is a general class of performance event.
//
// This corresponds to the perf_type_id enum from
// include/uapi/linux/perf_event.h
type EventType uint32

//go:generate stringer -type=EventType

const (
	EventTypeHardware EventType = iota
	EventTypeSoftware
	EventTypeTracepoint
	EventTypeHWCache
	EventTypeRaw
	EventTypeBreakpoint
)

// An EventID combined with an EventType describes a specific event.
type EventID uint64

// EventAttr describes an event and how that event should be recorded.
//
// This corresponds to the perf_event_attr struct from
// include/uapi/linux/perf_event.h
type EventAttr struct {
	// Event describes the event that will be (or was) counted or
	// sampled.
	Event Event

	// SamplePeriod, if non-zero, is the approximate number of
	// events between each sample.
	//
	// For a sampled event, SamplePeriod will be set if
	// Flags&EventFlagsFreq == 0. See also SampleFreq.
	SamplePeriod uint64

	// SampleFreq, if non-zero, is the approximate number of
	// samples to record per second per core. This is approximated
	// by dynamically adjusting the event sampling period (see
	// perf_calculate_period) and thus is not particularly
	// accurate (and even less accurate for events that don't
	// happen at a regular rate). If SampleFormat includes
	// SampleFormatPeriod, each sample includes the number of
	// events until the next sample on the same CPU.
	//
	// For a sampled event, SampleFreq will be set if
	// Flags&EventFlagsFreq != 0. See also SamplePeriod.
	SampleFreq uint64

	// The format of RecordSamples
	SampleFormat SampleFormat

	// The format of SampleRead
	ReadFormat ReadFormat

	Flags EventFlags

	// Precise indicates the precision of instruction pointers
	// recorded by this event.
	Precise EventPrecision

	// WakeupEvents specifies to wake up every WakeupEvents
	// events. Either this or WakeupWatermark will be non-zero,
	// depending on Flags&EventFlagWakeupWatermark.
	WakeupEvents uint32
	// WakeupWatermark specifies to wake up every WakeupWatermark
	// bytes.
	WakeupWatermark uint32

	BranchSampleType uint64 // TODO: PERF_SAMPLE_BRANCH_*

	// SampleRegsUser is a bitmask of user-space registers
	// captured at each sample in RecordSample.RegsUser. The
	// hardware register corresponding to each bit depends on the
	// register ABI.
	SampleRegsUser uint64

	// Size of user stack to dump on samples
	SampleStackUser uint32

	// SampleRegsIntr is a bitmask of registers captured at each
	// sample in RecordSample.RegsIntr. If Precise ==
	// EventPrecisionArbitrarySkid, these registers are captured
	// at the PMU interrupt. Otherwise, these registers are
	// captured by the hardware when it samples an instruction.
	SampleRegsIntr uint64

	// AuxWatermark is the watermark for the AUX area in bytes at
	// which user space is woken up to collect the AUX area.
	AuxWatermark uint32
}

// A SampleFormat is a bitmask of the fields recorded by a sample.
//
// This corresponds to the perf_event_sample_format enum from
// include/uapi/linux/perf_event.h
type SampleFormat uint64

//go:generate go run ../cmd/bitstringer/main.go -type=SampleFormat -strip=SampleFormat

const (
	SampleFormatIP SampleFormat = 1 << iota
	SampleFormatTID
	SampleFormatTime
	SampleFormatAddr
	SampleFormatRead
	SampleFormatCallchain
	SampleFormatID
	SampleFormatCPU
	SampleFormatPeriod
	SampleFormatStreamID
	SampleFormatRaw
	SampleFormatBranchStack
	SampleFormatRegsUser
	SampleFormatStackUser
	SampleFormatWeight
	SampleFormatDataSrc
	SampleFormatIdentifier
	SampleFormatTransaction
	SampleFormatRegsIntr
)

// sampleIDOffset returns the byte offset of the ID field within an
// on-disk sample record with this sample format. If there is no ID
// field, it returns -1.
func (s SampleFormat) sampleIDOffset() int {
	// See __perf_evsel__calc_id_pos in tools/perf/util/evsel.c.

	if s&SampleFormatIdentifier != 0 {
		return 0
	}
	if s&SampleFormatID == 0 {
		return -1
	}

	off := 0
	if s&SampleFormatIP != 0 {
		off += 8
	}
	if s&SampleFormatTID != 0 {
		off += 8
	}
	if s&SampleFormatTime != 0 {
		off += 8
	}
	if s&SampleFormatAddr != 0 {
		off += 8
	}
	return off
}

// recordIDOffset returns the byte offset of the ID field of
// non-sample records relative to the end of the on-disk sample. If
// there is no ID field, it returns -1.
func (s SampleFormat) recordIDOffset() int {
	// See __perf_evsel__calc_is_pos in tools/perf/util/evsel.c.

	if s&SampleFormatIdentifier != 0 {
		return -8
	}
	if s&SampleFormatID == 0 {
		return -1
	}

	off := 0
	if s&SampleFormatCPU != 0 {
		off -= 8
	}
	if s&SampleFormatStreamID != 0 {
		off -= 8
	}
	return off - 8
}

// trailerBytes returns the length in the sample_id trailer for
// non-sample records.
func (s SampleFormat) trailerBytes() int {
	s &= SampleFormatTID | SampleFormatTime | SampleFormatID | SampleFormatStreamID | SampleFormatCPU | SampleFormatIdentifier
	return 8 * weight(uint64(s))
}

// ReadFormat is a bitmask of the fields recorded in the SampleRead
// field(s) of a sample.
//
// This corresponds to the perf_event_read_format enum from
// include/uapi/linux/perf_event.h
type ReadFormat uint64

//go:generate go run ../cmd/bitstringer/main.go -type=ReadFormat -strip=ReadFormat

const (
	ReadFormatTotalTimeEnabled ReadFormat = 1 << iota
	ReadFormatTotalTimeRunning
	ReadFormatID
	ReadFormatGroup
)

// EventFlags is a bitmask of boolean properties of an event.
//
// This corresponds to the perf_event_attr enum from
// include/uapi/linux/perf_event.h
type EventFlags uint64

//go:generate go run ../cmd/bitstringer/main.go -type=EventFlags -strip=EventFlag

const (
	// Event is disabled by default
	EventFlagDisabled EventFlags = 1 << iota
	// Children inherit this event
	EventFlagInherit
	// Event must always be on the PMU
	EventFlagPinned
	// Event is only group on PMU
	EventFlagExclusive
	// Don't count events in user/kernel/hypervisor/when idle
	EventFlagExcludeUser
	EventFlagExcludeKernel
	EventFlagExcludeHypervisor
	EventFlagExcludeIdle
	// Include mmap data
	EventFlagMmap
	// Include comm data
	EventFlagComm
	// Use frequency, not period
	EventFlagFreq
	// Per task counts
	EventFlagInheritStat
	// Next exec enables this event
	EventFlagEnableOnExec
	// Trace fork/exit
	EventFlagTask
	// WakeupWatermark is set rather than WakeupEvents.
	EventFlagWakeupWatermark

	// Skip two bits here for EventFlagPreciseIPMask

	// Non-exec mmap data
	EventFlagMmapData EventFlags = 1 << (2 + iota)
	// All events have SampleField fields
	EventFlagSampleIDAll
	// Don't count events in host/guest
	EventFlagExcludeHost
	EventFlagExcludeGuest
	// Don't include kernel/user callchains
	EventFlagExcludeCallchainKernel
	EventFlagExcludeCallchainUser
	// Include inode data in mmap events
	EventFlagMmapInodeData
	// Flag comm events that are due to an exec
	EventFlagCommExec
	// Use clock specified by clockid for time fields
	EventFlagClockID

	eventFlagPreciseShift = 15
	eventFlagPreciseMask  = 0x3 << eventFlagPreciseShift
)

// An EventPrecision indicates the precision of instruction pointers
// recorded by an event. This can vary depending on the exact method
// used to capture IPs.
type EventPrecision int

//go:generate stringer -type=EventPrecision

const (
	EventPrecisionArbitrarySkid EventPrecision = iota
	EventPrecisionConstantSkid
	EventPrecisionTryZeroSkid
	EventPrecisionZeroSkip
)

// perf_event_header from include/uapi/linux/perf_event.h
type recordHeader struct {
	Type RecordType
	Misc recordMisc
	Size uint16
}

// A RecordType indicates the type of a record in a profile. A record
// can either be a profiling sample or give information about changes
// to system state, such as a process calling mmap.
type RecordType uint32

//go:generate stringer -type=RecordType

const (
	RecordTypeMmap RecordType = 1 + iota
	RecordTypeLost
	RecordTypeComm
	RecordTypeExit
	RecordTypeThrottle
	RecordTypeUnthrottle
	RecordTypeFork
	RecordTypeRead
	RecordTypeSample
	recordTypeMmap2 // internal extended RecordTypeMmap
	RecordTypeAux

	recordTypeUserStart RecordType = 64
)

// perf_user_event_type in tools/perf/util/event.h
//
// TODO: Figure out what to do with these. Some of these are only to
// direct parsing so they should never escape the API. Some of these
// are only for perf.data pipes.
const (
	recordTypeHeaderAttr      RecordType = recordTypeUserStart + iota
	recordTypeHeaderEventType            // deprecated
	recordTypeHeaderTracingData
	recordTypeHeaderBuildID
	recordTypeHeaderFinishedRound
	recordTypeHeaderIDIndex
)

// PERF_RECORD_MISC_* from include/uapi/linux/perf_event.h
type recordMisc uint16

const (
	recordMiscCPUModeMask recordMisc = 7
	recordMiscMmapData               = 1 << 13
	recordMiscCommExec               = 1 << 13
	recordMiscExactIP                = 1 << 14
)

// Record is the common interface implemented by all profile record
// types.
type Record interface {
	Type() RecordType
	Common() *RecordCommon
}

// RecordCommon stores fields that are common to all record types, as
// well as additional metadata. It is not itself a Record.
//
// Many fields are optional and their presence is determined by the
// bitmask EventAttr.SampleFormat. Some record types guarantee that
// some of these fields will be filled.
type RecordCommon struct {
	// Offset is the byte offset of this event in the perf.data
	// file.
	Offset int64

	// Format is a bit mask of SampleFormat* values that indicate
	// which optional fields of this record are valid.
	Format SampleFormat

	// EventAttr is the event, if any, associated with this record.
	EventAttr *EventAttr

	PID, TID int    // if SampleFormatTID
	Time     uint64 // if SampleFormatTime
	ID       attrID // if SampleFormatID or SampleFormatIdentifier
	StreamID uint64 // if SampleFormatStreamID
	CPU, Res uint32 // if SampleFormatCPU
}

func (r *RecordCommon) Common() *RecordCommon {
	return r
}

// A RecordUnknown is a Record of unknown or unimplemented type.
type RecordUnknown struct {
	recordHeader

	RecordCommon

	Data []byte
}

func (r *RecordUnknown) Type() RecordType {
	return RecordType(r.recordHeader.Type)
}

// A RecordMmap records when a process being profiled called mmap.
// RecordMmaps can also occur at the beginning of a profile to
// describe the existing memory layout.
type RecordMmap struct {
	// RecordCommon.PID and .TID will always be filled
	RecordCommon

	Data bool // from header.misc

	// Addr and Len are the virtual address of the start of this
	// mapping and its length in bytes.
	Addr, Len uint64
	// FileOffset is the byte offset in the mapped file of the
	// beginning of this mapping.
	FileOffset         uint64
	Major, Minor       uint32
	Ino, InoGeneration uint64
	Prot, Flags        uint32
	Filename           string
}

func (r *RecordMmap) Type() RecordType {
	return RecordTypeMmap
}

// A RecordLost records that profiling events were lost because of a
// buffer overflow.
type RecordLost struct {
	// RecordCommon.ID and .EventAttr will always be filled
	RecordCommon

	NumLost uint64
}

func (r *RecordLost) Type() RecordType {
	return RecordTypeLost
}

// A RecordComm records that a process being profiled called exec.
// RecordComms can also occur at the beginning of a profile to
// describe the existing set of processes.
type RecordComm struct {
	// RecordCommon.PID and .TID will always be filled
	RecordCommon

	Exec bool // from header.misc

	Comm string
}

func (r *RecordComm) Type() RecordType {
	return RecordTypeComm
}

// A RecordExit records that a process or thread exited.
type RecordExit struct {
	// RecordCommon.PID, .TID, and .Time will always be filled
	RecordCommon

	PPID, PTID int
}

func (r *RecordExit) Type() RecordType {
	return RecordTypeExit
}

// A RecordThrottle records that interrupt throttling was enabled or
// disabled.
type RecordThrottle struct {
	// RecordCommon.Time, .ID, and .StreamID, and .EventAttr will
	// always be filled
	RecordCommon

	Enable bool
}

func (r *RecordThrottle) Type() RecordType {
	return RecordTypeThrottle
}

// A RecordFork records that a process called clone to either fork the
// process or create a new thread.
type RecordFork struct {
	// RecordCommon.PID, .TID, and .Time will always be filled
	RecordCommon

	PPID, PTID int
}

func (r *RecordFork) Type() RecordType {
	return RecordTypeFork
}

// A RecordAux records the data was added to the AUX buffer.
type RecordAux struct {
	RecordCommon

	Offset, Size uint64
	Flags        AuxFlags
}

func (r *RecordAux) Type() RecordType {
	return RecordTypeAux
}

// AuxFlags gives flags for an RecordAux event.
type AuxFlags uint64

//go:generate go run ../cmd/bitstringer/main.go -type=AuxFlags -strip=AuxFlag

const (
	// Record was truncated to fit in the ring buffer.
	AuxFlagTruncated AuxFlags = 1 << iota

	// AUX data was collected in overwrite mode, so the AUX buffer
	// was treated as a circular ring buffer.
	AuxFlagOverwrite
)

// A RecordSample records a profiling sample event.
//
// Typically only a subset of the fields are used. Which fields are
// set can be determined from the bitmask
// RecordSample.EventAttr.SampleFormat.
type RecordSample struct {
	// RecordCommon.EventAttr will always be filled.
	// RecordCommon.Format descibes the optional fields in this
	// structure, as well as the optional common fields.
	RecordCommon

	CPUMode CPUMode // from header.misc
	ExactIP bool    // from header.misc

	IP   uint64 // if SampleFormatIP
	Addr uint64 // if SampleFormatAddr

	// Period is the number of events on this CPU until the next
	// sample. In frequency sampling mode, this is adjusted
	// dynamically based on the rate of recent events. In period
	// sampling mode, this is fixed.
	Period uint64 // if SampleFormatPeriod

	// SampleRead records raw event counter values. If this is an
	// event group, this slice will have more than one element;
	// otherwise, it will have one element.
	SampleRead []Count // if SampleFormatRead

	// Callchain gives the call stack of the sampled instruction,
	// starting from the sampled instruction itself. The call
	// chain may span several types of stacks (e.g., it may start
	// in a kernel stack, then transition to a user stack). Before
	// the first IP from each stack there will be a Callchain*
	// constant indicating the stack type for the following IPs.
	Callchain []uint64 // if SampleFormatCallchain

	BranchStack []BranchRecord // if SampleFormatBranchStack

	// RegsUserABI and RegsUser record the ABI and values of
	// user-space registers as of this sample. Note that these are
	// the current user-space registers even if this sample
	// occurred at a kernel PC. RegsUser[i] records the value of
	// the register indicated by the i-th set bit of
	// EventAttr.SampleRegsUser.
	RegsUserABI SampleRegsABI // if SampleFormatRegsUser
	RegsUser    []uint64      // if SampleFormatRegsUser

	// RegsIntrABI And RegsIntr record the ABI and values of
	// registers as of this sample. Unlike RegsUser, these can be
	// kernel-space registers if this sample occurs in the kernel.
	// RegsIntr[i] records the value of the register indicated by
	// the i-th set bit of EventAttr.SampleRegsIntr.
	RegsIntrABI SampleRegsABI // if SampleFormatRegsIntr
	RegsIntr    []uint64      // if SampleFormatRegsIntr

	StackUser        []byte // if SampleFormatStackUser
	StackUserDynSize uint64 // if SampleFormatStackUser

	Weight  uint64  // if SampleFormatWeight
	DataSrc DataSrc // if SampleFormatDataSrc

	Transaction Transaction // if SampleFormatTransaction
	AbortCode   uint32      // if SampleFormatTransaction
}

func (r *RecordSample) Type() RecordType {
	return RecordTypeSample
}

func (r *RecordSample) String() string {
	// TODO: Stringers for other record types
	f := r.Format
	s := fmt.Sprintf("{Offset:%v Format:%v EventAttr:%p CPUMode:%v ExactIP:%v", r.Offset, r.Format, r.EventAttr, r.CPUMode, r.ExactIP)
	if f&(SampleFormatID|SampleFormatIdentifier) != 0 {
		s += fmt.Sprintf(" ID:%d", r.ID)
	}
	if f&SampleFormatIP != 0 {
		s += fmt.Sprintf(" IP:%#x", r.IP)
	}
	if f&SampleFormatTID != 0 {
		s += fmt.Sprintf(" PID:%d TID:%d", r.PID, r.TID)
	}
	if f&SampleFormatTime != 0 {
		s += fmt.Sprintf(" Time:%d", r.Time)
	}
	if f&SampleFormatAddr != 0 {
		s += fmt.Sprintf(" Addr:%#x", r.Addr)
	}
	if f&SampleFormatStreamID != 0 {
		s += fmt.Sprintf(" StreamID:%d", r.StreamID)
	}
	if f&SampleFormatCPU != 0 {
		s += fmt.Sprintf(" CPU:%d Res:%d", r.CPU, r.Res)
	}
	if f&SampleFormatPeriod != 0 {
		s += fmt.Sprintf(" Period:%d", r.Period)
	}
	if f&SampleFormatRead != 0 {
		s += fmt.Sprintf(" SampleRead:%v", r.SampleRead)
	}
	if f&SampleFormatCallchain != 0 {
		s += fmt.Sprintf(" Callchain:%#x", r.Callchain)
	}
	if f&SampleFormatBranchStack != 0 {
		s += fmt.Sprintf(" BranchStack:%v", r.BranchStack)
	}
	if f&SampleFormatRegsUser != 0 {
		s += fmt.Sprintf(" RegsUserABI:%v RegsUser:%v", r.RegsUserABI, r.RegsUser)
	}
	if f&SampleFormatRegsIntr != 0 {
		s += fmt.Sprintf(" RegsIntrABI:%v RegsIntr:%v", r.RegsIntrABI, r.RegsIntr)
	}
	if f&SampleFormatStackUser != 0 {
		s += fmt.Sprintf(" StackUser:[...] StackUserDynSize:%d", r.StackUserDynSize)
	}
	if f&SampleFormatWeight != 0 {
		s += fmt.Sprintf(" Weight:%d", r.Weight)
	}
	if f&SampleFormatDataSrc != 0 {
		s += fmt.Sprintf(" DataSrc:%+v", r.DataSrc)
	}
	if f&SampleFormatTransaction != 0 {
		s += fmt.Sprintf(" Transaction:%v AbortCode:%d", r.Transaction, r.AbortCode)
	}
	return s + "}"
}

// Fields returns the list of names of valid fields in r based on
// r.Format. This is useful for writing custom printing functions.
func (r *RecordSample) Fields() []string {
	f := r.Format
	fs := []string{"Offset", "Format", "EventAttr", "CPUMode", "ExactIP"}
	if f&(SampleFormatID|SampleFormatIdentifier) != 0 {
		fs = append(fs, "ID")
	}
	if f&SampleFormatIP != 0 {
		fs = append(fs, "IP")
	}
	if f&SampleFormatTID != 0 {
		fs = append(fs, "PID", "TID")
	}
	if f&SampleFormatTime != 0 {
		fs = append(fs, "Time")
	}
	if f&SampleFormatAddr != 0 {
		fs = append(fs, "Addr")
	}
	if f&SampleFormatStreamID != 0 {
		fs = append(fs, "StreamID")
	}
	if f&SampleFormatCPU != 0 {
		fs = append(fs, "CPU", "Res")
	}
	if f&SampleFormatPeriod != 0 {
		fs = append(fs, "Period")
	}
	if f&SampleFormatRead != 0 {
		fs = append(fs, "SampleRead")
	}
	if f&SampleFormatCallchain != 0 {
		fs = append(fs, "Callchain")
	}
	if f&SampleFormatBranchStack != 0 {
		fs = append(fs, "BranchStack")
	}
	if f&SampleFormatRegsUser != 0 {
		fs = append(fs, "RegsUserABI", "RegsUser")
	}
	if f&SampleFormatRegsIntr != 0 {
		fs = append(fs, "RegsIntrABI", "RegsIntr")
	}
	if f&SampleFormatStackUser != 0 {
		fs = append(fs, "StackUser", "StackUserDynSize")
	}
	if f&SampleFormatWeight != 0 {
		fs = append(fs, "Weight")
	}
	if f&SampleFormatDataSrc != 0 {
		fs = append(fs, "DataSrc")
	}
	if f&SampleFormatTransaction != 0 {
		fs = append(fs, "Transaction", "AbortCode")
	}
	return fs
}

// A CPUMode indicates the privilege level of a sample or event.
//
// This corresponds to PERF_RECORD_MISC_CPUMODE from
// include/uapi/linux/perf_event.h
type CPUMode uint16

//go:generate stringer -type=CPUMode

const (
	CPUModeUnknown CPUMode = iota
	CPUModeKernel
	CPUModeUser
	CPUModeHypervisor
	CPUModeGuestKernel
	CPUModeGuestUser
)

// A Count records the raw value of an event counter.
//
// Typically only a subset of the fields are used. Which fields are
// set can be determined from the bitmask in the sample's
// EventAttr.ReadFormat.
//
// This corresponds to perf_event_read_format from
// include/uapi/linux/perf_event.h
type Count struct {
	Value       uint64     // Event counter value
	TimeEnabled uint64     // if ReadFormatTotalTimeEnabled
	TimeRunning uint64     // if ReadFormatTotalTimeRunning
	EventAttr   *EventAttr // if ReadFormatID
}

// A BranchRecord records a single branching event in a sample.
type BranchRecord struct {
	From, To uint64
	Flags    BranchFlags
}

type BranchFlags uint64

//go:generate go run ../cmd/bitstringer/main.go -type=BranchFlags -strip=BranchFlag

const (
	// BranchFlagMispredicted indicates branch target was mispredicted.
	BranchFlagMispredicted BranchFlags = 1 << iota

	// BranchFlagPredicted indicates branch target was predicted.
	// In case predicted/mispredicted information is unavailable,
	// both flags will be unset.
	BranchFlagPredicted

	// BranchFlagInTransaction indicates the branch occurred in a
	// transaction.
	BranchFlagInTransaction

	// BranchFlagAbort indicates the branch is a transaction abort.
	BranchFlagAbort
)

// Special markers used in RecordSample.Callchain to mark boundaries
// between types of stacks.
//
// These correspond to PERF_CONTEXT_* from
// include/uapi/linux/perf_event.h
const (
	CallchainHypervisor  = 0xffffffffffffffe0 // -32
	CallchainKernel      = 0xffffffffffffff80 // -128
	CallchainUser        = 0xfffffffffffffe00 // -512
	CallchainGuest       = 0xfffffffffffff800 // -2048
	CallchainGuestKernel = 0xfffffffffffff780 // -2176
	CallchainGuestUser   = 0xfffffffffffff600 // -2560
)

// SampleRegsABI indicates the register ABI of a given sample for
// architectures that support multiple ABIs.
//
// This corresponds to the perf_sample_regs_abi enum from
// include/uapi/linux/perf_event.h
type SampleRegsABI uint64

//go:generate stringer -type=SampleRegsABI

const (
	SampleRegsABINone SampleRegsABI = iota
	SampleRegsABI32
	SampleRegsABI64
)

type DataSrc struct {
	Op     DataSrcOp
	Miss   bool // if true, Level specifies miss, rather than hit
	Level  DataSrcLevel
	Snoop  DataSrcSnoop
	Locked DataSrcLock
	TLB    DataSrcTLB
}

type DataSrcOp int

//go:generate go run ../cmd/bitstringer/main.go -type=DataSrcOp -strip=DataSrcOp

const (
	DataSrcOpLoad DataSrcOp = 1 << iota
	DataSrcOpStore
	DataSrcOpPrefetch
	DataSrcOpExec

	DataSrcOpNA DataSrcOp = 0
)

type DataSrcLevel int

//go:generate go run ../cmd/bitstringer/main.go -type=DataSrcLevel -strip=DataSrcLevel

const (
	DataSrcLevelL1  DataSrcLevel = 1 << iota
	DataSrcLevelLFB              // Line fill buffer
	DataSrcLevelL2
	DataSrcLevelL3
	DataSrcLevelLocalRAM     // Local DRAM
	DataSrcLevelRemoteRAM1   // Remote DRAM (1 hop)
	DataSrcLevelRemoteRAM2   // Remote DRAM (2 hops)
	DataSrcLevelRemoteCache1 // Remote cache (1 hop)
	DataSrcLevelRemoteCache2 // Remote cache (2 hops)
	DataSrcLevelIO           // I/O memory
	DataSrcLevelUncached

	DataSrcLevelNA DataSrcLevel = 0
)

type DataSrcSnoop int

//go:generate go run ../cmd/bitstringer/main.go -type=DataSrcSnoop -strip=DataSrcSnoop

const (
	DataSrcSnoopNone DataSrcSnoop = 1 << iota
	DataSrcSnoopHit
	DataSrcSnoopMiss
	DataSrcSnoopHitM // Snoop hit modified

	DataSrcSnoopNA DataSrcSnoop = 0
)

type DataSrcLock int

//go:generate stringer -type=DataSrcLock

const (
	DataSrcLockNA DataSrcLock = iota
	DataSrcLockUnlocked
	DataSrcLockLocked
)

type DataSrcTLB int

//go:generate go run ../cmd/bitstringer/main.go -type=DataSrcTLB -strip=DataSrcTLB

const (
	DataSrcTLBHit DataSrcTLB = 1 << iota
	DataSrcTLBMiss
	DataSrcTLBL1
	DataSrcTLBL2
	DataSrcTLBHardwareWalker
	DataSrcTLBOSFaultHandler

	DataSrcTLBNA DataSrcTLB = 0
)

type Transaction int

//go:generate go run ../cmd/bitstringer/main.go -type=Transaction -strip=Transaction

const (
	TransactionElision       Transaction = 1 << iota // From elision
	TransactionTransaction                           // From transaction
	TransactionSync                                  // Instruction is related
	TransactionAsync                                 // Instruction is not related
	TransactionRetry                                 // Retry possible
	TransactionConflict                              // Conflict abort
	TransactionCapacityWrite                         // Capactiy write abort
	TransactionCapacityRead                          // Capactiy read abort
)
