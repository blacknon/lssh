package psrp

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

const (
	DefaultProtocolVersion      = "2.3"
	DefaultSerializationVersion = "1.1.0.1"
	DefaultPSVersion            = "2.0"
	CreateShellProtocolVersion  = "2.2"
)

type RunspacePoolInit struct {
	MinRunspaces int
	MaxRunspaces int
	Width        int
	Height       int
}

func DefaultRunspacePoolInit() RunspacePoolInit {
	return RunspacePoolInit{
		MinRunspaces: 1,
		MaxRunspaces: 1,
		Width:        120,
		Height:       40,
	}
}

func BuildSessionCapabilityData(protocolVersion, psVersion, serializationVersion string) []byte {
	if protocolVersion == "" {
		protocolVersion = DefaultProtocolVersion
	}
	if psVersion == "" {
		psVersion = DefaultPSVersion
	}
	if serializationVersion == "" {
		serializationVersion = DefaultSerializationVersion
	}

	return []byte(fmt.Sprintf(
		`<Obj RefId="0"><MS><Version N="protocolversion">%s</Version><Version N="PSVersion">%s</Version><Version N="SerializationVersion">%s</Version><Nil N="TimeZone" /><B N="MustComply">true</B></MS></Obj>`,
		protocolVersion,
		psVersion,
		serializationVersion,
	))
}

func BuildInitRunspacePoolData(init RunspacePoolInit) []byte {
	if init.MinRunspaces <= 0 {
		init.MinRunspaces = 1
	}
	if init.MaxRunspaces <= 0 {
		init.MaxRunspaces = init.MinRunspaces
	}
	if init.Width <= 0 {
		init.Width = 120
	}
	if init.Height <= 0 {
		init.Height = 40
	}

	return []byte(fmt.Sprintf(
		`<Obj RefId="0"><MS>`+
			`<I32 N="MinRunspaces">%d</I32>`+
			`<I32 N="MaxRunspaces">%d</I32>`+
			`<Obj N="PSThreadOptions" RefId="1"><TN RefId="0"><T>System.Management.Automation.Runspaces.PSThreadOptions</T><T>System.Enum</T><T>System.ValueType</T><T>System.Object</T></TN><ToString>Default</ToString><I32>0</I32></Obj>`+
			`<Obj N="ApartmentState" RefId="2"><TN RefId="1"><T>System.Threading.ApartmentState</T><T>System.Enum</T><T>System.ValueType</T><T>System.Object</T></TN><ToString>Unknown</ToString><I32>2</I32></Obj>`+
			buildApplicationArgumentsXML()+
			`<Obj N="HostInfo" RefId="6"><MS>`+
			buildHostDefaultDataXML(init.Width, init.Height)+
			`<B N="_isHostNull">false</B><B N="_isHostUINull">false</B><B N="_isHostRawUINull">false</B><B N="_useRunspaceHost">false</B>`+
			`</MS></Obj>`+
			`</MS></Obj>`,
		init.MinRunspaces,
		init.MaxRunspaces,
	))
}

func buildApplicationArgumentsXML() string {
	return `<Obj N="ApplicationArguments" RefId="3">` +
		`<TN RefId="2"><T>System.Management.Automation.PSPrimitiveDictionary</T><T>System.Collections.Hashtable</T><T>System.Object</T></TN>` +
		`<DCT>` +
		`<En><S N="Key">PSVersionTable</S>` +
		`<Obj N="Value" RefId="4"><TNRef RefId="2" /><DCT>` +
		buildVersionTableEntry("PSVersion", "2.0", true) +
		buildStringTableEntry("PSEdition", "Desktop") +
		buildVersionArrayTableEntry("PSCompatibleVersions", []string{"1.0", "2.0", "3.0", "4.0", "5.0", "5.1.0.0"}) +
		buildVersionTableEntry("CLRVersion", "4.0.30319.42000", true) +
		buildVersionTableEntry("BuildVersion", "10.0.0.0", true) +
		buildVersionTableEntry("WSManStackVersion", "3.0", true) +
		buildVersionTableEntry("PSRemotingProtocolVersion", DefaultProtocolVersion, true) +
		buildVersionTableEntry("SerializationVersion", DefaultSerializationVersion, true) +
		`</DCT></Obj></En>` +
		`</DCT></Obj>`
}

func buildVersionTableEntry(key, value string, named bool) string {
	if named {
		return fmt.Sprintf(`<En><S N="Key">%s</S><Version N="Value">%s</Version></En>`, key, value)
	}
	return fmt.Sprintf(`<En><S N="Key">%s</S><Version>%s</Version></En>`, key, value)
}

func buildStringTableEntry(key, value string) string {
	return fmt.Sprintf(`<En><S N="Key">%s</S><S N="Value">%s</S></En>`, key, value)
}

func buildVersionArrayTableEntry(key string, versions []string) string {
	var builder strings.Builder
	builder.WriteString(`<En><S N="Key">`)
	builder.WriteString(key)
	builder.WriteString(`</S><Obj N="Value" RefId="5"><TN RefId="4"><T>System.Version[]</T><T>System.Array</T><T>System.Object</T></TN><LST>`)
	for _, version := range versions {
		builder.WriteString(`<Version>`)
		builder.WriteString(version)
		builder.WriteString(`</Version>`)
	}
	builder.WriteString(`</LST></Obj></En>`)
	return builder.String()
}

func buildHostDefaultDataXML(width, height int) string {
	const (
		foregroundColor = 6
		backgroundColor = 1
		cursorX         = 0
		cursorY         = 17
		windowPosX      = 0
		windowPosY      = 0
		cursorSize      = 25
		maxPhysicalWidth = 239
		maxPhysicalHeight = 91
		maxWindowHeight = 91
		windowHeight = 50
		bufferHeight = 6000
		windowTitle = "Administrator: Windows PowerShell"
	)

	type entry struct {
		key   int
		value string
	}

	entries := []entry{
		{key: 0, value: buildIntValueXML(7, "System.ConsoleColor", foregroundColor)},
		{key: 1, value: buildIntValueXML(8, "System.ConsoleColor", backgroundColor)},
		{key: 2, value: buildCoordinatesValueXML(9, 10, cursorX, cursorY)},
		{key: 3, value: buildCoordinatesValueXML(11, 12, windowPosX, windowPosY)},
		{key: 4, value: buildIntValueXML(13, "System.Int32", cursorSize)},
		{key: 5, value: buildSizeValueXML(14, 15, width, bufferHeight)},
		{key: 6, value: buildSizeValueXML(16, 17, width, windowHeight)},
		{key: 7, value: buildSizeValueXML(18, 19, width, maxWindowHeight)},
		{key: 8, value: buildSizeValueXML(20, 21, maxPhysicalWidth, maxPhysicalHeight)},
		{key: 9, value: buildStringValueXML(22, windowTitle)},
	}

	var builder strings.Builder
	builder.WriteString(`<Obj N="_hostDefaultData" RefId="7"><MS>`)
	builder.WriteString(`<Obj N="data" RefId="8"><TN RefId="3"><T>System.Collections.Hashtable</T><T>System.Object</T></TN><DCT>`)
	for _, entry := range entries {
		builder.WriteString(`<En><I32 N="Key">`)
		builder.WriteString(fmt.Sprintf("%d", entry.key))
		builder.WriteString(`</I32>`)
		builder.WriteString(entry.value)
		builder.WriteString(`</En>`)
	}
	builder.WriteString(`</DCT></Obj></MS></Obj>`)
	return builder.String()
}

func buildIntValueXML(refID int, typeName string, value int) string {
	return fmt.Sprintf(`<Obj N="Value" RefId="%d"><MS><S N="T">%s</S><I32 N="V">%d</I32></MS></Obj>`, refID, typeName, value)
}

func buildCoordinatesValueXML(valueRefID, coordRefID, x, y int) string {
	return fmt.Sprintf(`<Obj N="Value" RefId="%d"><MS><S N="T">System.Management.Automation.Host.Coordinates</S><Obj N="V" RefId="%d"><MS><I32 N="x">%d</I32><I32 N="y">%d</I32></MS></Obj></MS></Obj>`, valueRefID, coordRefID, x, y)
}

func buildSizeValueXML(valueRefID, sizeRefID, width, height int) string {
	return fmt.Sprintf(`<Obj N="Value" RefId="%d"><MS><S N="T">System.Management.Automation.Host.Size</S><Obj N="V" RefId="%d"><MS><I32 N="width">%d</I32><I32 N="height">%d</I32></MS></Obj></MS></Obj>`, valueRefID, sizeRefID, width, height)
}

func buildStringValueXML(refID int, value string) string {
	return fmt.Sprintf(`<Obj N="Value" RefId="%d"><MS><S N="T">System.String</S><S N="V">%s</S></MS></Obj>`, refID, value)
}

func BuildOpenRunspacePoolCreationXML(runspacePoolID uuid.UUID, init RunspacePoolInit) ([]byte, error) {
	sessionCapability := EncodeMessage(Message{
		Destination:    DestinationServer,
		Type:           MessageSessionCapability,
		RunspacePoolID: runspacePoolID,
		PipelineID:     uuid.Nil,
		Data:           BuildSessionCapabilityData("", "", ""),
	})

	initRunspacePool := EncodeMessage(Message{
		Destination:    DestinationServer,
		Type:           MessageInitRunspacePool,
		RunspacePoolID: runspacePoolID,
		PipelineID:     uuid.Nil,
		Data:           BuildInitRunspacePoolData(init),
	})

	fragments := append(
		FragmentMessage(1, sessionCapability, 0),
		FragmentMessage(2, initRunspacePool, 0)...,
	)

	return BuildCreationXML(fragments)
}
