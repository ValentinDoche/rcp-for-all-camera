package panasonic

import (
	"fmt"
	"strings"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// AW-UE150 control set.
//
// Source: SKAARHOJ core-panasonic-ptz parameter list for the AW-UE150,
// https://cloud.skaarhoj.com/parameterlist/core-panasonic-ptz/pre/2
// (fetched 2026-10-11). Row format: see dsl.go. Option codes are the numeric
// values of the list ("0:Off;1:On").
//
// Left out on purpose, as for the UE160: Config (CameraName, Firmware,
// connection, device_ip, device_test), dcTrace* and dcCruiseControl,
// PresetThumbnail, PresetName, PresetExists, the *Incremental parameters, the
// duplicate ColorTemperatureValue, and the raw CamIris / IrisPosition scales.
// The UE150 list has no ColorGainG, no Tally Yellow and 16 color correction
// axes (four more than the UE160).

var ue150Colors = [][2]string{
	{"BMg", "B-Mg"}, {"B", "B"}, {"CyB", "Cy-B"}, {"Cy", "Cy"}, {"GCy", "G-Cy"}, {"G", "G"},
	{"Mg", "Mg"}, {"MgR", "Mg-R"}, {"MgRR", "Mg-R-R"}, {"R", "R"}, {"RRYI", "R-R-YI"},
	{"RYI", "R-YI"}, {"RYIYI", "R-YI-YI"}, {"YIG", "YI-G"}, {"YI", "YI"}, {"YIYIG", "YI-YI-G"},
}

var ue150StdParams = stdParams{
	gainLo: 0, gainHi: 42, // codes 8..50, then 128 = Auto
	shutter:  []string{"1/60", "1/100", "1/120", "1/250", "1/500", "1/1000", "1/2000", "1/4000", "1/8000", "1/10000"},
	pedestal: 200, pedestalRGB: 100, colorGain: 200,
	chromaMin: -99, chromaMax: 99,
}

func ue150Specs() []core.ControlSpec {
	s := standardControls(ue150StdParams)
	var b strings.Builder
	for _, p := range []string{"BG", "BR", "GB", "GR", "RB", "RG"} {
		fmt.Fprintf(&b, "i|LinearMatrix%s|Linear matrix %s-%s|color|Matrix linear|-63|63|\n", p, p[:1], p[1:])
	}
	s = append(s, parseRows(b.String())...)
	s = append(s, parseRows(colorCorrectRows(ue150Colors, -63, 63))...)
	s = append(s, parseRows(ue150Rows)...)
	s = append(s, presetNumberSpecs()...)
	return s
}

const ue150Rows = `
# Audio
o|AudioEnable|Audio enable|audio|Audio|0:Off;1:On
o|AudioInputVolume=audioInputType|Audio input type|audio|Audio|0:Mic;3:Line
i|AudioOutputVolume|Audio CH1 volume|audio|Audio|-36|12|dB
o|AudioPluginPower|Audio plug-in power|audio|Audio|0:Off;1:On

# Color: chroma, matrix, pedestal
o|AdaptiveMatrix|Adaptive matrix|color|Matrix|0:Off;1:On
o|ColorMatrix|Color matrix|color|Matrix|0:Normal;1:EBU;2:NTSC;3:User
i|ChromaPhase|Chroma phase|color|Chroma|-31|31
o|PedestalOffset|Pedestal offset|color|Pedestal|0:Off;1:On

# Color bars
o|ColorBarTone|Color bars tone|image|Color bars|0:Off;1:Low;2:Normal
o|ColorBarType|Color bars type|image|Color bars|0:TYPE2 (BAR/EBU);1:TYPE1 (SMPTE)

# Crop
o|CropIpOut|Crop IP 1 output|image|Crop output|0:Full;1:Crop
o|CropSDIOut|Crop 3G SDI 1 output|image|Crop output|0:Full;1:Crop
i|CropHPositionG|Crop position H green|image|Crop position|0|2560
i|CropHPositionMG|Crop position H magenta|image|Crop position|0|2560
i|CropHPositionYL|Crop position H yellow|image|Crop position|0|2560
i|CropVPositionG|Crop position V green|image|Crop position|0|1440
i|CropVPositionMG|Crop position V magenta|image|Crop position|0|1440
i|CropVPositionYL|Crop position V yellow|image|Crop position|0|1440
i|CropHSpeed|Crop position speed H|image|Crop speed|1|99
i|CropVSpeed|Crop position speed V|image|Crop speed|1|99
o|CropAdjSelect|Crop adjust select|image|Crop placement|1:Yellow;2:Green;3:Magenta
o|CropMarkerSelect|Crop marker select|image|Crop placement|0:Off;1:YL;2:G;3:MG;4:YL+G;5:YL+MG;6:G+MG;7:YL+G+MG
o|CropOutSelect|Crop output select|image|Crop placement|1:Yellow;2:Green;3:Magenta
o|CropUHD|Crop UHD|image|Crop placement|0:Off;1:Crop 1080;2:Crop 720

# Detail
i|DetailBand|Detail frequency band|image|Detail|-7|7
o|DetailControl|Detail control|image|Detail|0:Off;1:On
i|DetailCoring|Detail coring|image|Detail|0|60
i|DetailMinus|Detail gain -|image|Detail|-31|31
i|DetailPlus|Detail gain +|image|Detail|-31|31
i|KneeApertureLevel|Knee aperture level|image|Detail|0|5
i|LevelDependentValue|Level dependent value|image|Detail|-7|7
i|VDtlLevel|V detail level|image|Detail|-7|7

# Digital extender / zoom
o|DigitalExtender|Digital extender mode|image|Digital extender|0:Off;1:x1.4;2:x2.0
o|DigitalZoom|Digital zoom|image|Digital zoom|0:Off;1:On
f|DigitalZoomMag|Digital zoom magnification|image|Digital zoom|1|10|0.1|x
i|DigitalZoomMax|Digital zoom maximum|image|Digital zoom|2|10|x
o|DigitaliZoomEnable|Digital iZoom|image|Digital zoom|0:Off;1:On

# Exposure
o|FrameMixControl|Frame mix|exposure|Frame mix|0:Off;6:6dB;12:12dB;18:18dB;24:24dB
o|AGCMax|AGC max|exposure|Gain|1:6dB;2:12dB;3:18dB
o|GainSuper|Super gain|exposure|Gain|0:Off;1:On
o|ShutterLimit|Shutter limit|exposure|Shutter|2:1/100;3:1/120;4:1/250
o|ShutterMode|Shutter mode|exposure|Shutter|0:Off;1:Step;2:Synchro;3:ELC
f|ShutterSynchroControl|Shutter synchro|exposure|Shutter|24|7200|0.1|Hz

# External output
o|ExternalOutput1|External output 1|system|External output|0:Off;1:Red Tally;2:Green Tally
o|ExternalOutput2|External output 2|system|External output|0:Off;1:Red Tally;2:Green Tally

# Frequency
o|Frequency|Video frequency|output|Frequency|0:59.94Hz;1:50Hz;2:24Hz;3:23.98Hz

# Gamma
o|BlackGammaRange|Black gamma range|image|Black gamma|1:1;2:2;3:3
i|MasterBlackGamma|Master black gamma|image|Black gamma|-8|8
i|FRECBlackStrLVL|F-Rec black stretch level|image|F-Rec|0|30
o|FRECDynamicLVL|F-Rec dynamic level|image|F-Rec|0:200%;1:300%;2:400%;3:500%;4:600%
f|GammaLevelControl=gammaLevel|Gamma level|image|Gamma|0.3|0.75|0.01
i|VRECKneePoint|V-Rec knee point|image|V-Rec|30|107|%
i|VRECKneeSlope|V-Rec knee slope|image|V-Rec|150|500|%
o|DRSControl|DRS control|image|Gamma|0:Off;1:Low;2:Mid;3:High
o|GammaType|Gamma type|image|Gamma|0:HD;2:FilmLike1;3:FilmLike2;4:FilmLike3;5:Film Rec;6:Video Rec;7:HLG

# Housing and installation
o|Fan1|Fan 1|system|Housing|0:Auto;1:High;2:Mid;3:Low
o|Fan2|Fan 2|system|Housing|0:Auto;1:High;2:Mid;3:Low
o|POSITION|Installation position|system|Installation|0:Desktop;1:Hanging
o|RemoteController|Wireless remote|system|Installation|0:Disable;1:Enable
o|SmartFlip|Smart flip status|system|Installation|0:Normal;1:Flip
i|SmartFlipAngle|Smart picture flip angle|system|Installation|60|120|deg
o|SmartPictureFlip|Smart picture flip|system|Installation|0:Off;1:Auto
o|StatusLamp|Status lamp|system|Installation|0:Disable;1:Enable

# Knee
o|KneeHLG|Knee HLG|image|Knee HLG|0:Off;1:On
f|KneePointHLG|Knee point HLG|image|Knee HLG|55|100|0.01|%
i|KneeSlopeHLG|Knee slope HLG|image|Knee HLG|0|100
i|KneeAuto|Knee auto|image|Knee|1|8
o|KneeMode|Knee mode|image|Knee|0:Off;1:Manual;2:Auto
f|KneePointMaster|Knee point master|image|Knee|70|107|0.1|%
i|KneeSlopeMaster|Knee slope master|image|Knee|0|99

# Lens
o|CamFocusAuto=focusMode|Focus mode|lens|Focus|0:Manual;1:Auto
i|FocusMaxSpeed|Focus max speed|lens|Focus|5|100|%
i|FocusSpeed|Focus speed|lens|Focus|1|99
o|CamIrisAuto=irisMode|Iris mode|lens|Iris|0:Manual;1:Auto
i|CamIrisAutoAdjust=irisAutoOffset|Auto iris offset|lens|Iris|-50|50
o|CamIrisAutoSpeed=irisAutoSpeed|Auto iris speed|lens|Iris|0:Slow;1:Normal;2:Fast
o|CamIrisAutoWindow=irisAutoWindow|Auto iris window|lens|Iris|0:Normal 1;1:Normal 2;2:Center

# Movement range, night mode, noise reduction, OIS
o|MovementRangeLimit|Movement range limit|ptz|Movement range|0:Release;1:Set
o|NightMode|Night mode|image|Night mode|0:Off;1:On
o|DNRenabled=dnrMode|DNR|image|Noise reduction|0:Off;1:Low;2:High
o|OISSettings|Optical image stabilizer|image|OIS|0:Off;1:On

# OSD menu
t|OSDMenuCancel|OSD menu cancel|system|OSD menu
t|OSDMenuDown|OSD menu down / no|system|OSD menu
o|OSDMenuEnable|OSD menu|system|OSD menu|0:Off;1:On
t|OSDMenuEnter|OSD menu enter|system|OSD menu
t|OSDMenuLeft|OSD menu left|system|OSD menu
o|OSDMenuOffRedTally|OSD off R-tally|system|OSD menu|0:Off;1:On
t|OSDMenuRight|OSD menu right|system|OSD menu
t|OSDMenuUp|OSD menu up / yes|system|OSD menu

# Output settings
o|12GSDIHDROut|12G SDI HDR output|output|12G SDI|0:SDR;1:HDR(2020);2:HDR(709)
o|12GSDIOutLevel|12G SDI level|output|12G SDI|0:Level A;1:Level B
o|12GSDIOutputFormat|12G SDI output format|output|12G SDI|1:720/59.94p;2:720/50p;4:1080/59.94i;5:1080/50i;7:1080/29.97PsF;8:1080/25PsF;10:1080/23.98PsF;16:1080/59.94p;17:1080/50p;20:1080/29.97p;21:1080/25p;22:1080/23.98p;23:2160/29.97p;24:2160/25p;25:2160/59.94p;26:2160/50p;27:2160/23.98p;33:2160/24p;34:1080/24p;35:1080/23.98p
o|3GSDIHDROut|3G SDI HDR output|output|3G SDI|0:SDR;1:HDR(2020);2:HDR(709)
o|3GSDIOutLevel|3G SDI level|output|3G SDI|0:Level A;1:Level B
o|HDMIColorComponent|HDMI color component|output|HDMI|2:YCbCr422;4:YPbPr420
o|HDMIHDROut|HDMI HDR output|output|HDMI|0:SDR;1:HDR(2020);2:HDR(709)
o|HDMIOutputFormat|HDMI output format|output|HDMI|1:720/59.94p;2:720/50p;4:1080/59.94i;5:1080/50i;16:1080/59.94p;17:1080/50p;20:1080/29.97p;21:1080/25p;22:1080/23.98p;23:2160/29.97p;24:2160/25p;25:2160/59.94p;26:2160/50p;27:2160/23.98p;33:2160/24p;34:1080/24p;35:1080/23.98p
o|MonitorSDIHDROut|Monitor SDI HDR output|output|Monitor SDI|0:SDR;1:HDR(2020);2:HDR(709)
o|MonitorSDIOutputFormat|Monitor SDI output format|output|Monitor SDI|1:720/59.94p;2:720/50p;4:1080/59.94i;5:1080/50i;7:1080/29.97PsF;8:1080/25PsF;10:1080/23.98PsF;20:1080/29.97p;21:1080/25p;22:1080/23.98p;34:1080/24p;35:1080/23.98p
o|FormatSDI=sdiOutputFormat|3G SDI output format|output|SDI|1:720/59.94p;2:720/50p;4:1080/59.94i;5:1080/50i;7:1080/29.97PsF;8:1080/25PsF;10:1080/23.98PsF;16:1080/59.94p;17:1080/50p;20:1080/29.97p;21:1080/25p;22:1080/23.98p;34:1080/24p;35:1080/23.98p
o|Format=systemFormat|System format|output|System|1:720/59.94p;2:720/50p;4:1080/59.94i;5:1080/50i;7:1080/29.97PsF;8:1080/25PsF;10:1080/23.98PsF;16:1080/59.94p;17:1080/50p;20:1080/29.97p;21:1080/25p;22:1080/23.98p;23:2160/29.97p;24:2160/25p;25:2160/59.94p;26:2160/50p;27:2160/23.98p;33:2160/24p;34:1080/24p;35:1080/23.98p

# Power
o|POWER|Power|system|Power|0:Standby;1:Power On
o|PowerOnPosition|Power-on position|system|Power|0:None;1:Standby;2:Home;3:Preset
i|PowerOnPreset|Power-on preset|system|Power|1|100

# Presets
o|PresetCrop|Preset crop|ptz|Preset options|0:Off;1:On
o|PresetFreeze|Preset freeze|ptz|Preset options|0:Off;1:On
o|PresetIris|Preset iris|ptz|Preset options|0:Off;1:On
o|PresetPlayback=presetScope|Preset scope|ptz|Preset options|0:Mode A;1:Mode B;2:Mode C
i|PresetSpeed|Preset speed|ptz|Preset options|1|30
o|PresetSpeedMode|Preset speed mode|ptz|Preset options|0:Speed Table;1:Time
i|PresetSpeedSec|Preset speed time|ptz|Preset options|1|99|s
o|PresetSpeedTable|Preset speed table|ptz|Preset options|0:Slow;2:High
o|PresetZoom|Preset zoom|ptz|Preset options|0:Mode A;1:Mode B
t|presetHome|Home position|ptz|Presets

# PTZ
i|panspeed=panSpeed|Pan speed|ptz|Speed|1|99
i|tiltspeed=tiltSpeed|Tilt speed|ptz|Speed|1|99
i|zoomspeed=zoomSpeed|Zoom speed|ptz|Speed|1|99
o|FocusADJWithPTZ=focusAdjustWithPtz|Focus adjust with PTZ|ptz|PTZ operation|0:Off;1:On
o|PTSpeedControl=ptSpeedMode|Pan/tilt speed mode|ptz|PTZ operation|0:Normal (60deg/s);1:Fast (180deg/s);2:Quick (60deg/s)
o|PTZoomPosLinked=ptZoomPositionLinked|Pan/tilt zoom position linked|ptz|PTZ operation|0:Off;1:On

# Scene
o|SceneFile|Scene file|system|Scene files|1:Scene 1;2:Scene 2;3:Scene 3;4:Scene 4

# Shooting mode
o|ShootingMode|Shooting mode|image|Shooting mode|0:Normal;1:Hi Sens.

# Skin
o|SkinToneDetail|Skin tone detail|image|Skin tone|0:Off;1:On
i|SkinToneMode|Skin tone mode|image|Skin tone|0|31

# Tally (settable: modelled as driver controls, not the read-only standard tally)
o|TallyBrightness|Tally brightness|system|Tally|0:Low;1:Mid;2:High
o|TallyEnable|Tally enable|system|Tally|0:Disable;1:Enable
o|TallyG|Tally green|system|Tally|0:Off;1:On
o|TallyR|Tally red|system|Tally|0:Off;1:On

# Tracking data
o|TrackingDataIP|Tracking data IP|system|Tracking data|0:Off;1:On
o|TrackingDataSerial|Tracking data serial|system|Tracking data|0:Off;1:On

# V-Log
o|ColorSetting|Color setting|output|V-Log|0:Normal;1:V-Log
o|ColorSetting12G|Color setting 12G|output|V-Log|0:V-Log;1:V-709
o|ColorSetting3G|Color setting 3G|output|V-Log|0:V-Log;1:V-709
o|ColorSettingHDMI|Color setting HDMI|output|V-Log|0:V-Log;1:V-709
o|ColorSettingMonitor|Color setting monitor|output|V-Log|0:V-Log;1:V-709

# White balance
t|ABBExec|ABB execute|white|ABB
o|ATWSpeed|ATW speed|white|AWB|0:Normal;1:Slow;2:Fast
i|ATWTargetB|ATW target B|white|AWB|-10|10
i|ATWTargetR|ATW target R|white|AWB|-10|10
o|AWBExecDisp|AWB status display|white|AWB|0:Off;1:On
o|AWBGainOffset|AWB gain offset|white|AWB|0:Off;1:On
o|AWBMode|White balance mode|white|AWB|0:ATW;1:AWB A;2:AWB B;4:Preset 3200K;5:Preset 5600K;9:VAR
i|AWB_BGain=awbBGain|AWB B gain|white|AWB|-400|400
i|AWB_GAxis=awbGAxis|AWB G axis|white|AWB|-400|400
i|AWB_RGain=awbRGain|AWB R gain|white|AWB|-400|400
t|ColorTemperatureDown|Color temperature down|white|AWB
t|ColorTemperatureUp|Color temperature up|white|AWB
o|WhiteClip|White clip|white|White clip|0:Off;1:On
i|WhiteClipLevel|White clip level|white|White clip|90|109|%
`
