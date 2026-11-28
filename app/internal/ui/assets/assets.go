package assets

import (
	"bytes"
	"embed"
	"image/png"

	"github.com/egoist/mygo/ui"
)

//go:embed samples/*.png
var samples embed.FS

func SamplePicture(lang string) []byte {
	b, _ := samples.ReadFile("samples/dialogue-" + lang + ".png")
	return b
}

var (
	//go:embed icon.png
	AppIconPNG []byte
	AppIcon, _ = ui.DecodeBitmap(AppIconPNG)
	//go:embed sticker.png
	stickerPNG []byte
	Sticker, _ = ui.DecodeBitmap(stickerPNG)
	//go:embed sticker-asleep.png
	stickerAsleepPNG []byte
	StickerAsleep, _ = ui.DecodeBitmap(stickerAsleepPNG)
	StickerAspect    = aspect(stickerPNG)

	//go:embed tray-idle.png
	TrayIdle []byte
	//go:embed tray-on.png
	TrayOn []byte

	//go:embed icons/lock.svg
	lockSVG []byte
	//go:embed icons/settings.svg
	settingsSVG []byte
	//go:embed icons/pause.svg
	pauseSVG []byte
	//go:embed icons/play.svg
	playSVG []byte
	//go:embed icons/arrow-left-right.svg
	swapSVG []byte

	Lock, Settings    = ui.MustParseSVG(lockSVG), ui.MustParseSVG(settingsSVG)
	Pause, Play, Swap = ui.MustParseSVG(pauseSVG), ui.MustParseSVG(playSVG), ui.MustParseSVG(swapSVG)

	//go:embed icons/volume-2.svg
	soundSVG []byte
	//go:embed icons/audio-lines.svg
	wavesSVG []byte
	//go:embed icons/mouse-pointer-2.svg
	pointerSVG []byte
	//go:embed icons/cloud.svg
	cloudSVG []byte
	//go:embed icons/captions.svg
	captionsSVG []byte
	//go:embed icons/square.svg
	stopSVG []byte
	//go:embed icons/keyboard.svg
	keyboardSVG []byte

	Sound, Waves, Pointer = ui.MustParseSVG(soundSVG), ui.MustParseSVG(wavesSVG), ui.MustParseSVG(pointerSVG)
	Cloud, Captions, Stop = ui.MustParseSVG(cloudSVG), ui.MustParseSVG(captionsSVG), ui.MustParseSVG(stopSVG)
	Keyboard              = ui.MustParseSVG(keyboardSVG)

	//go:embed icons/plus.svg
	plusSVG []byte
	//go:embed icons/arrow-right.svg
	arrowSVG []byte
	//go:embed icons/trash-2.svg
	trashSVG []byte
	//go:embed icons/rotate-ccw.svg
	resetSVG []byte
	//go:embed icons/sparkles.svg
	sparklesSVG []byte
	//go:embed icons/check.svg
	checkSVG []byte
	//go:embed icons/download.svg
	downloadSVG []byte
	//go:embed icons/sliders-horizontal.svg
	slidersSVG []byte
	//go:embed icons/pencil.svg
	pencilSVG []byte
	//go:embed icons/ellipsis.svg
	moreSVG []byte

	Plus, Arrow, Trash, Reset = ui.MustParseSVG(plusSVG), ui.MustParseSVG(arrowSVG), ui.MustParseSVG(trashSVG), ui.MustParseSVG(resetSVG)
	Sparkles, Check, Download = ui.MustParseSVG(sparklesSVG), ui.MustParseSVG(checkSVG), ui.MustParseSVG(downloadSVG)
	Sliders, Pencil, More     = ui.MustParseSVG(slidersSVG), ui.MustParseSVG(pencilSVG), ui.MustParseSVG(moreSVG)
	//go:embed icons/folder-open.svg
	folderSVG []byte
	Folder    = ui.MustParseSVG(folderSVG)

	//go:embed icons/house.svg
	homeSVG []byte
	//go:embed icons/languages.svg
	languagesSVG []byte
	//go:embed icons/book.svg
	bookSVG []byte
	//go:embed icons/history.svg
	historySVG []byte
	//go:embed icons/info.svg
	infoSVG []byte
	//go:embed icons/chart-column.svg
	chartSVG []byte

	Home, Languages, Book = ui.MustParseSVG(homeSVG), ui.MustParseSVG(languagesSVG), ui.MustParseSVG(bookSVG)
	History, Info, Chart  = ui.MustParseSVG(historySVG), ui.MustParseSVG(infoSVG), ui.MustParseSVG(chartSVG)

	//go:embed icons/chevron-down.svg
	chevronSVG []byte
	Chevron    = ui.MustParseSVG(chevronSVG)
	//go:embed icons/move.svg
	moveSVG []byte
	//go:embed icons/minus.svg
	minusSVG    []byte
	Move, Minus = ui.MustParseSVG(moveSVG), ui.MustParseSVG(minusSVG)
	//go:embed icons/zap.svg
	zapSVG []byte
	Zap    = ui.MustParseSVG(zapSVG)

	//go:embed icons/scan-text.svg
	scanSVG []byte
	//go:embed icons/lock-open.svg
	unlockSVG []byte
	//go:embed icons/power.svg
	quitSVG []byte

	Scan, Unlock, Quit = ui.MustParseSVG(scanSVG), ui.MustParseSVG(unlockSVG), ui.MustParseSVG(quitSVG)

	//go:embed icons/x.svg
	closeSVG []byte
	Close    = ui.MustParseSVG(closeSVG)

	//go:embed icons/image.svg
	imageSVG []byte
	//go:embed icons/image-off.svg
	imageOffSVG []byte
	//go:embed icons/copy.svg
	copySVG []byte
	//go:embed icons/zoom-in.svg
	zoomInSVG []byte

	Picture, Copy = ui.MustParseSVG(imageSVG), ui.MustParseSVG(copySVG)
	NoPicture     = ui.MustParseSVG(imageOffSVG)
	ZoomIn        = ui.MustParseSVG(zoomInSVG)

	//go:embed icons/arrow-up-right.svg
	externalSVG []byte
	External    = ui.MustParseSVG(externalSVG)

	//go:embed icons/external-link.svg
	opensSVG []byte
	Opens    = ui.MustParseSVG(opensSVG)

	//go:embed icons/globe.svg
	globeSVG []byte
	//go:embed icons/github.svg
	gitHubSVG     []byte
	Globe, GitHub = ui.MustParseSVG(globeSVG), ui.MustParseSVG(gitHubSVG)

	Tray, _      = ui.DecodeBitmap(TrayIdle)
	TrayAwake, _ = ui.DecodeBitmap(TrayOn)
)

func aspect(b []byte) float32 {
	cfg, err := png.DecodeConfig(bytes.NewReader(b))
	if err != nil || cfg.Height == 0 {
		return 1
	}
	return float32(cfg.Width) / float32(cfg.Height)
}
