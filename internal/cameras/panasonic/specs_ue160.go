package panasonic

import (
	"fmt"
	"strings"

	"github.com/valentindoche/rcp-for-all-camera/internal/core"
)

// AW-UE160 control set.
//
// Source: SKAARHOJ core-panasonic-ptz parameter list for the AW-UE160,
// https://cloud.skaarhoj.com/parameterlist/core-panasonic-ptz/pre/18
// (fetched 2026-10-11). Row format: see dsl.go. Option codes are the numeric
// values of the list ("0:Off;1:On").
//
// Left out on purpose: Config (CameraName, Firmware, connection, device_ip,
// device_test), dcTrace* and dcCruiseControl (SKAARHOJ trace engine),
// PresetThumbnail, PresetName, PresetExists, the *Incremental parameters, the
// duplicate ColorTemperatureValue, and the raw CamIris / IrisPosition scales
// (CamIrisFvalue is exposed as iris).

var ue160Colors = [][2]string{
	{"BMg", "B-Mg"}, {"B", "B"}, {"CyB", "Cy-B"}, {"Cy", "Cy"}, {"GCy", "G-Cy"}, {"G", "G"},
	{"Mg", "Mg"}, {"MgR", "Mg-R"}, {"R", "R"}, {"RYI", "R-YI"}, {"YIG", "YI-G"}, {"YI", "YI"},
}

var ue160StdParams = stdParams{
	gainLo: -6, gainHi: 12, // codes 2..20, then 128 = Auto
	shutter:  []string{"1/60", "1/100", "1/125", "1/250", "1/500", "1/1000", "1/1500", "1/2000"},
	pedestal: 800, pedestalRGB: 800, colorGain: 1000,
	chromaMin: -100, chromaMax: 40, greenColorGain: true,
}

func ue160Specs() []core.ControlSpec {
	s := standardControls(ue160StdParams)
	s = append(s, linearMatrixSpecs(31)...)
	s = append(s, parseRows(colorCorrectRows(ue160Colors, -127, 126))...)
	s = append(s, parseRows(ue160Rows)...)
	s = append(s, presetNumberSpecs()...)
	return s
}

// linearMatrixSpecs builds the signed linear matrix terms (N and P side).
func linearMatrixSpecs(lim int) []core.ControlSpec {
	var b strings.Builder
	for _, p := range []string{"BG", "BR", "GB", "GR", "RB", "RG"} {
		for _, side := range []struct{ id, label string }{{"N", "-"}, {"P", "+"}} {
			fmt.Fprintf(&b, "i|LinearMatrix%s_%s|Linear matrix %s-%s (%s)|color|Matrix linear|%d|%d|\n",
				p, side.id, p[:1], p[1:], side.label, -lim, lim)
		}
	}
	return parseRows(b.String())
}

// presetNumberSpecs are PresetStore and PresetDelete: choosing a preset
// number executes the action on that preset (nothing is sent until a value is
// chosen).
func presetNumberSpecs() []core.ControlSpec {
	st := core.ControlSpec{ID: "panasonic.presetStore", Label: "Store preset", Group: core.GroupPTZ,
		Section: "Presets", Kind: core.KindEnum, Options: presetOptions(100)}
	del := core.ControlSpec{ID: "panasonic.presetDelete", Label: "Delete preset", Group: core.GroupPTZ,
		Section: "Presets", Kind: core.KindEnum, Options: presetOptions(100)}
	return []core.ControlSpec{st, del}
}

const ue160Rows = `
# Audio
o|AudioEnable|Audio enable|audio|Audio|0:Off;1:On
o|AudioHeadRoom|Audio head room|audio|Audio|0:FS-12dB;1:FS-18dB;2:FS-20dB
o|AudioInputSelect|Audio input type|audio|Audio|0:Line;1:Mic;2:Mic+48v
o|AudioLineInLevel|Audio line input level|audio|Audio|0:+4dB;1:0dB
o|AudioMicGain|Audio mic gain|audio|Audio|0:60dB;1:40dB
o|AudioOuputCHSelect=audioOutputChannel|Audio output channel|audio|Audio|0:CH1/CH2;1:CH1;2:CH2
i|AudioOutputVolume|Audio output volume|audio|Audio|-40|20|dB

# Color: chroma, matrix, pedestal
o|ChromaLevelEnable|Chroma level enable|color|Chroma|0:Off;1:On
o|ColorCorrectCtrl|Color correct matrix|color|Matrix|0:Off;1:On
o|ColorCorrectTableCtrl|Color correct table|color|Matrix|0:A;1:B
o|ColorMatrix|Color matrix|color|Matrix|0:Normal;1:Cinema 1;2:Cinema 2;3:User;4:HD
o|ColorMatrixCtrl|Linear color matrix|color|Matrix|0:Off;1:On
o|ColorMatrixTableCtrl|Linear color matrix table|color|Matrix|0:A;1:B
o|MatrixCtrl|Matrix on output|color|Matrix|0:Off;1:On
o|PedestalOffset|Pedestal offset|color|Pedestal|0:Off;1:On

# Color bars
o|ColorBarTone|Color bars tone|image|Color bars|0:Off;1:On
o|ColorBarType|Color bars type|image|Color bars|0:TYPE2 (FULL);1:TYPE1 (SMPTE);2:TYPE3 (ARIB/MULTI)

# Crop
o|CropIp2Out|Crop IP 2 output|image|Crop output|0:Full;1:Crop
o|CropIpOut|Crop IP 1 output|image|Crop output|0:Full;1:Crop
o|CropNdiOut|Crop NDI output|image|Crop output|0:Full;1:Crop
o|CropSDIOut|Crop 3G SDI 1 output|image|Crop output|0:Full;1:Crop
i|CropHPosition|Crop position H|image|Crop position|0|3072
i|CropHPositionG|Crop position H green|image|Crop position|0|3072
i|CropHPositionMG|Crop position H magenta|image|Crop position|0|3072
i|CropHPositionYL|Crop position H yellow|image|Crop position|0|3072
i|CropVPosition|Crop position V|image|Crop position|0|1728
i|CropVPositionG|Crop position V green|image|Crop position|0|1728
i|CropVPositionMG|Crop position V magenta|image|Crop position|0|1728
i|CropVPositionYL|Crop position V yellow|image|Crop position|0|1728
i|CropHSpeed|Crop position speed H|image|Crop speed|1|99
i|CropVSpeed|Crop position speed V|image|Crop speed|1|99
f|CropZoomRatio|Crop zoom ratio|image|Crop zoom|120|500|0.01|%
f|CropZoomRatioG|Crop zoom ratio green|image|Crop zoom|120|500|0.01|%
f|CropZoomRatioMG|Crop zoom ratio magenta|image|Crop zoom|120|500|0.01|%
f|CropZoomRatioYL|Crop zoom ratio yellow|image|Crop zoom|120|500|0.01|%
i|CropZoomSpeed|Crop zoom speed|image|Crop zoom|1|99
i|CropZoomSpeedG|Crop zoom speed green|image|Crop zoom|1|99
i|CropZoomSpeedMG|Crop zoom speed magenta|image|Crop zoom|1|99
i|CropZoomSpeedYL|Crop zoom speed yellow|image|Crop zoom|1|99
o|CropZoom|Crop zoom|image|Crop zoom|0:Off;1:On
o|CropAF|Crop autofocus|image|Crop zoom|0:Off;1:On
o|CropAdjSelect|Crop adjust select|image|Crop zoom|1:Yellow;2:Green;3:Magenta
o|CropMarkerSelect|Crop marker select|image|Crop zoom|0:Off;1:YL;2:G;3:MG;4:YL+G;5:YL+MG;6:G+MG;7:YL+G+MG
o|CropOutSelect|Crop output select|image|Crop zoom|1:Yellow;2:Green;3:Magenta
o|CropUHD|Crop UHD|image|Crop zoom|0:Off;1:Crop 1080;2:Crop 720

# Detail
o|DarkDetailSwitch|Dark detail switch|image|Detail|0:Off;1:On
i|DarkDetailValue|Dark detail|image|Detail|0|7
o|DetailControl|Detail control|image|Detail|0:Off;1:On
i|DetailKnee|Detail knee|image|Detail|0|15
i|DetailMinus|Detail gain -|image|Detail|-31|31
i|DetailMinusClip|Detail - clip|image|Detail|0|63
i|DetailPlus|Detail gain +|image|Detail|-31|31
i|DetailPlusClip|Detail + clip|image|Detail|0|63
i|KneeApertureLevel|Knee aperture level|image|Detail|0|39
o|LevelDependentEnable|Level dependent|image|Detail|0:Off;1:On
i|LevelDependentValue|Level dependent value|image|Detail|0|15
i|NoiseSuppress=detailCrisp|Detail crisp|image|Detail|0|63
i|PeakFrequency|Peak frequency|image|Detail|1|8

# Digital extender / zoom
o|DigitalExtender|Digital extender mode|image|Digital extender|0:Off;1:x1.4;2:x2.0
o|DigitalZoom|Digital zoom|image|Digital zoom|0:Off;1:On
f|DigitalZoomMag|Digital zoom magnification|image|Digital zoom|1|10|0.1|x
i|DigitalZoomMax|Digital zoom maximum|image|Digital zoom|2|10|x
o|DigitaliZoomEnable|Digital iZoom|image|Digital zoom|0:Off;1:On

# Exposure
o|FrameMixControl|Frame mix|exposure|Frame mix|0:Off;6:6dB;12:12dB;18:18dB;24:24dB
o|AGCMax|AGC max|exposure|Gain|1:6dB;2:12dB
o|ShutterLimit|Shutter limit|exposure|Shutter|2:1/100;3:1/120;4:1/250
o|ShutterMode|Shutter mode|exposure|Shutter|0:Off;1:Step;2:Synchro;3:ELC
f|ShutterSynchroControl|Shutter synchro|exposure|Shutter|24|7200|0.1|Hz

# External output
o|ExternalOutput1|External output 1|system|External output|0:Off;1:Red Tally;2:Green Tally
o|ExternalOutput2|External output 2|system|External output|0:Off;1:Red Tally;2:Green Tally

# Flare
i|FlareB|Flare blue|image|Flare|-200|200
i|FlareG|Flare green|image|Flare|-200|200
i|FlareMaster|Flare master|image|Flare|-200|200
i|FlareR|Flare red|image|Flare|-200|200
o|FlareSW|Flare switch|image|Flare|0:Off;1:On

# Frequency
o|Frequency|Video frequency|output|Frequency|0:59.94Hz;1:50Hz;2:24Hz;3:23.98Hz;4:60Hz

# Gamma
i|BBlackGamma|B black gamma|image|Black gamma|-20|20
o|BlackGamma|Black gamma enable|image|Black gamma|0:Off;1:On
o|BlackGammaRange|Black gamma range|image|Black gamma|1:1;2:2;3:3
i|MasterBlackGamma|Master black gamma|image|Black gamma|-48|48
i|RBlackGamma|R black gamma|image|Black gamma|-20|20
i|FRECBlackStrLVL|F-Rec black stretch level|image|F-Rec|0|30
o|FRECDynamicLVL|F-Rec dynamic level|image|F-Rec|0:200%;1:300%;2:400%;3:500%;4:600%
i|BGamma|B gamma|image|Gamma|-75|75
f|GammaLevelControl=gammaLevel|Master gamma|image|Gamma|0.15|0.75|0.01
i|RGamma|R gamma|image|Gamma|-75|75
i|VRECKneePoint|V-Rec knee point|image|V-Rec|30|107|%
i|VRECKneeSlope|V-Rec knee slope|image|V-Rec|150|500|%
o|DRSControl|DRS control|image|Gamma|0:Off;1:On
o|ExtendedDRS=drsEffectDepth|DRS effect depth|image|Gamma|1:1;2:2;3:3;4:4;5:5
o|Gamma|Gamma|image|Gamma|0:Off;1:On
o|GammaType|Gamma mode|image|Gamma|0:HD;1:Normal;2:Cinema 1;3:Cinema 2

# HDR (HLG)
o|HLGBlack|HLG black gamma|output|HDR HLG|0:Off;1:On
i|HLGBlackBlue|HLG blue black gamma|output|HDR HLG|-32|32
i|HLGBlackMaster|HLG master black gamma|output|HDR HLG|-32|32
i|HLGBlackRed|HLG red black gamma|output|HDR HLG|-32|32
o|HLGMode|HLG mode|output|HDR HLG|0:Fix;1:Var
o|HLGSDRConvertGain|HLG SDR convert gain|output|HDR HLG|116:-12 dB;119:-9 dB;122:-6 dB;125:-3 dB;128:0 dB
o|HLGsdrConverter=hlgSdrConverter|HLG SDR converter|output|HDR HLG|0:Fix;1:Var
o|HDRswMain=hdrSwitchMain|HDR switch main|output|HDR|0:Off;1:On

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
f|KneePointHLG|Knee point HLG|image|Knee HLG|60|100|0.01|%
i|KneeSlopeHLG|Knee slope HLG|image|Knee HLG|0|199
i|KneeAuto|Knee auto|image|Knee|1|8
o|KneeMode|Knee mode|image|Knee|0:Off;1:Manual;2:Auto
i|KneePointBlue|Knee point blue|image|Knee|-25|25|%
f|KneePointMaster|Knee point master|image|Knee|70|107|0.1|%
i|KneePointRed|Knee point red|image|Knee|-25|25|%
i|KneeSlopeBlue|Knee slope blue|image|Knee|-99|99
i|KneeSlopeMaster|Knee slope master|image|Knee|0|199
i|KneeSlopeRed|Knee slope red|image|Knee|-99|99

# Lens
o|CamFocusAuto=focusMode|Focus mode|lens|Focus|0:Manual;1:Auto
i|FocusMaxSpeed|Focus max speed|lens|Focus|5|100|%
i|FocusSpeed|Focus speed|lens|Focus|1|99
o|CamIrisAuto=irisMode|Iris mode|lens|Iris|0:Manual;1:Auto
i|CamIrisAutoAdjust=irisAutoOffset|Auto iris offset|lens|Iris|-50|50
o|CamIrisAutoSpeed=irisAutoSpeed|Auto iris speed|lens|Iris|0:Slow;1:Normal;2:Fast
o|CamIrisAutoWindow=irisAutoWindow|Auto iris window|lens|Iris|1:1;2:2;3:3;4:4

# Movement range, night mode, noise reduction, OIS
o|MovementRangeLimit|Movement range limit|ptz|Movement range|0:Release;1:Set
o|NightMode|Night mode|image|Night mode|0:Off;1:On
o|DNRenabled=dnrMode|DNR|image|Noise reduction|0:Off;1:On;2:On 2
i|DNRlevel=dnrLevel|DNR level|image|Noise reduction|1|5
o|OISSettings|Optical image stabilizer|image|OIS|0:Off;1:On

# OSD menu
t|OSDMenuCancel|OSD menu cancel|system|OSD menu
t|OSDMenuDown|OSD menu down / no|system|OSD menu
o|OSDMenuEnable|OSD menu|system|OSD menu|0:Off;1:On
t|OSDMenuEnter|OSD menu enter|system|OSD menu
t|OSDMenuLeft|OSD menu left|system|OSD menu
t|OSDMenuRight|OSD menu right|system|OSD menu
t|OSDMenuUp|OSD menu up / yes|system|OSD menu

# Output settings
o|12GSDIHDROut|12G SDI HDR output|output|12G SDI|0:SDR;1:HDR(2020);2:HDR(709)
o|12GSDIOutLevel|12G SDI level|output|12G SDI|0:Level A;1:Level B
o|12GSDIOutputFormat|12G SDI output format|output|12G SDI|1:720/59.94p;2:720/50p;4:1080/59.94i;5:1080/50i;16:1080/59.94p;17:1080/50p;20:1080/29.97p;21:1080/25p;23:2160/29.97p;24:2160/25p;25:2160/59.94p;26:2160/50p;27:2160/23.98p;31:2160/60p;32:1080/60p;33:2160/24p;34:1080/24p;35:1080/23.98p
o|3GSDIHDROut|3G SDI HDR output|output|3G SDI|0:SDR;1:HDR(2020);2:HDR(709)
o|3GSDIOutLevel|3G SDI level|output|3G SDI|0:Level A;1:Level B
o|HDMIColorComponent|HDMI color component|output|HDMI|2:YCbCr422;4:YPbPr420
o|HDMIHDROut|HDMI HDR output|output|HDMI|0:SDR;1:HDR(2020);2:HDR(709)
o|HDMIOutputFormat|HDMI output format|output|HDMI|1:720/59.94p;2:720/50p;4:1080/59.94i;5:1080/50i;16:1080/59.94p;17:1080/50p;20:1080/29.97p;21:1080/25p;22:1080/23.98p;23:2160/29.97p;24:2160/25p;25:2160/59.94p;26:2160/50p;27:2160/23.98p;32:1080/60p;33:2160/24p;34:1080/24p;35:1080/23.98p;38:1080/119.88p;39:1080/100p
o|MonitorSDIHDROut|Monitor SDI HDR output|output|Monitor SDI|0:SDR;1:HDR(2020);2:HDR(709)
o|MonitorSDIOutputFormat|Monitor SDI 2 output format|output|Monitor SDI|1:720/59.94p;2:720/50p;4:1080/59.94i;5:1080/50i;16:1080/59.94p;17:1080/50p;20:1080/29.97p;21:1080/25p;32:1080/60p;34:1080/24p;35:1080/23.98p
o|FormatSDI=sdiOutputFormat|3G SDI 1 output format|output|SDI|1:720/59.94p;2:720/50p;4:1080/59.94i;5:1080/50i;16:1080/59.94p;17:1080/50p;20:1080/29.97p;21:1080/25p;32:1080/60p;34:1080/24p;35:1080/23.98p
o|Format=systemFormat|System format|output|System|1:720/59.94p;2:720/50p;16:1080/59.94p;17:1080/50p;20:1080/29.97p;21:1080/25p;23:2160/29.97p;24:2160/25p;25:2160/59.94p;26:2160/50p;27:2160/23.98p;32:1080/60p;33:2160/24p;34:1080/24p;35:1080/23.98p;38:1080/119.88p;39:1080/100p

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
o|PresetSpeedTable|Preset speed table|ptz|Preset options|0:Slow;2:Fast
o|PresetZoom|Preset zoom|ptz|Preset options|0:Mode A;1:Mode B
t|presetHome|Home position|ptz|Presets

# PTZ
i|panspeed=panSpeed|Pan speed|ptz|Speed|1|99
i|tiltspeed=tiltSpeed|Tilt speed|ptz|Speed|1|99
i|zoomspeed=zoomSpeed|Zoom speed|ptz|Speed|1|99
o|FocusADJWithPTZ=focusAdjustWithPtz|Focus adjust with PTZ|ptz|PTZ operation|0:Off;1:On
o|PTSpeedControl=ptSpeedMode|Pan/tilt speed mode|ptz|PTZ operation|0:Normal (60deg/s);1:Fast 1 (90deg/s);2:Fast 2 (180deg/s)
o|PTZoomPosLinked=ptZoomPositionLinked|Pan/tilt zoom position linked|ptz|PTZ operation|0:Off;1:On

# Scene and user files
o|SceneFile|Scene file load|system|Scene files|1:Scene 1;2:Scene 2;3:Scene 3;4:Scene 4;5:Scene 5;6:Scene 6;7:Scene 7;8:Scene 8;9:Off
t|SceneFileStore|Scene file store|system|Scene files
t|UserFile|User file load|system|Scene files
t|UserFileStore|User file store|system|Scene files

# Shooting mode
o|ShootingMode|Shooting mode|image|Shooting mode|0:Normal;1:Hi Sens.

# Skin
i|SkinToneCrisp|Skin tone crisp|image|Skin tone|-63|63
i|SkinToneDTLICenter|Skin tone detail I center|image|Skin tone|0|255
i|SkinToneDTLIWidth|Skin tone detail I width|image|Skin tone|0|255
i|SkinToneDTLQPhase|Skin tone detail Q phase|image|Skin tone|0|359
i|SkinToneDTLQWidth|Skin tone detail Q width|image|Skin tone|0|255
o|SkinToneDetail|Skin tone detail|image|Skin tone|0:Off;1:On
o|SkinToneEffectMemory|Skin tone effect memory|image|Skin tone|0:A;1:B;2:C;3:A+B;4:A+C;5:B+C;6:A+B+C
o|SkinToneZebra|Skin tone zebra|image|Skin tone|0:Off;1:On
o|SkinZebraEffectMemory|Skin zebra effect memory|image|Skin tone|0:A;1:B;2:C;3:A+B;4:A+C;5:B+C;6:A+B+C

# Tally (settable: modelled as driver controls, not the read-only standard tally)
o|TallyBrightness|Tally brightness|system|Tally|0:Low;1:Mid;2:High
o|TallyEnable|Tally enable|system|Tally|0:Disable;1:Enable
o|TallyG|Tally green|system|Tally|0:Off;1:On
o|TallyR|Tally red|system|Tally|0:Off;1:On
o|TallyY|Tally yellow|system|Tally|0:Off;1:On

# Tracking data
o|TrackingDataIP|Tracking data IP|system|Tracking data|0:Off;1:On
o|TrackingDataSerial|Tracking data serial|system|Tracking data|0:Off;1:On

# V-Log
o|ColorSetting|V-Log|output|V-Log|0:Off;1:On
o|ColorSetting12G|Color setting 12G|output|V-Log|0:V-Log;1:V-709
o|ColorSetting3G|Color setting 3G|output|V-Log|0:V-Log;1:V-709
o|ColorSettingHDMI|Color setting HDMI|output|V-Log|0:V-Log;1:V-709
o|ColorSettingMonitor|Color setting monitor|output|V-Log|0:V-Log;1:V-709
o|VLogPaintSw=vlogPaintSwitch|V-Log paint switch|output|V-Log|0:Off;1:On

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
i|WhiteClipLevel|White clip level|white|White clip|80|109|%
`
