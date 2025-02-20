package storage

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"NanoKVM-Server/proto"
)

const (
	imageDirectory = "/data"
	imageNone      = "/dev/mmcblk0p3"
	cdromFlag      = "/sys/kernel/config/usb_gadget/g0/functions/mass_storage.disk0/lun.0/cdrom"
	mountDevice    = "/sys/kernel/config/usb_gadget/g0/functions/mass_storage.disk0/lun.0/file"
	removableFlag  = "/sys/kernel/config/usb_gadget/g0/functions/mass_storage.disk0/lun.0/removable"
	roFlag         = "/sys/kernel/config/usb_gadget/g0/functions/mass_storage.disk0/lun.0/ro"
	roDisk         = "/boot/usb.disk0.ro"
)

func (s *Service) GetImages(c *gin.Context) {
	var rsp proto.Response
	var images []string

	err := filepath.Walk(imageDirectory, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			name := strings.ToLower(info.Name())
			if strings.HasSuffix(name, ".iso") || strings.HasSuffix(name, ".img") {
				images = append(images, path)
			}
		}

		return nil
	})
	if err != nil {
		rsp.ErrRsp(c, -2, "get images failed")
		return
	}

	rsp.OkRspWithData(c, &proto.GetImagesRsp{
		Files: images,
	})
	log.Debugf("get images success, total %d", len(images))
}

func (s *Service) MountImage(c *gin.Context) {
	var req proto.MountImageReq
	var rsp proto.Response

	if err := proto.ParseFormRequest(c, &req); err != nil {
		rsp.ErrRsp(c, -1, "invalid arguments")
		return
	}

	image := req.File
	if image == "" {
		image = imageNone
	}

	imageRemovable := "1"

	imageRo := "0"
	isImageRo, _ := isFlagExist(roDisk)
	if isImageRo {
		imageRo = "1"
	}

	imageLow := strings.ToLower(image)
	imageCdrom := "0"
	if strings.HasSuffix(imageLow, ".iso") {
		imageRo = "1"
		imageCdrom = "1"
	}

	// unmount
	if err := os.WriteFile(mountDevice, []byte("\n"), 0o666); err != nil {
		log.Errorf("unmount file failed: %s", err)
		rsp.ErrRsp(c, -2, "unmount image failed")
		return
	}

	// removable flag
	if err := os.WriteFile(removableFlag, []byte(imageRemovable), 0o666); err != nil {
		log.Errorf("set removable flag failed: %s", err)
		rsp.ErrRsp(c, -2, "set removable flag failed")
		return
	}

	// ro flag
	if err := os.WriteFile(roFlag, []byte(imageRo), 0o666); err != nil {
		log.Errorf("set ro flag failed: %s", err)
		rsp.ErrRsp(c, -2, "set ro flag failed")
		return
	}

	// cdrom flag
	if err := os.WriteFile(cdromFlag, []byte(imageCdrom), 0o666); err != nil {
		log.Errorf("set cdrom flag failed: %s", err)
		rsp.ErrRsp(c, -2, "set cdrom flag failed")
		return
	}

	// mount
	if err := os.WriteFile(mountDevice, []byte(image), 0o666); err != nil {
		log.Errorf("mount file %s failed: %s", image, err)
		rsp.ErrRsp(c, -2, "mount image failed")
		return
	}

	// reset usb
	commands := []string{
		"echo > /sys/kernel/config/usb_gadget/g0/UDC",
		"ls /sys/class/udc/ | cat > /sys/kernel/config/usb_gadget/g0/UDC",
	}

	for _, command := range commands {
		err := exec.Command("sh", "-c", command).Run()
		if err != nil {
			rsp.ErrRsp(c, -2, "execute command failed")
			return
		}
	}

	rsp.OkRsp(c)
	log.Debugf("mount image %s success", req.File)
}

func (s *Service) GetMountedImage(c *gin.Context) {
	var rsp proto.Response

	content, err := os.ReadFile(mountDevice)
	if err != nil {
		rsp.ErrRsp(c, -2, "read failed")
		return
	}

	image := strings.ReplaceAll(string(content), "\n", "")
	if image == imageNone {
		image = ""
	}

	data := &proto.GetMountedImageRsp{
		File: image,
	}

	rsp.OkRspWithData(c, data)
}

func isFlagExist(flag string) (bool, error) {
	_, err := os.Stat(flag)

	if err == nil {
		return true, nil
	}

	if errors.Is(err, os.ErrNotExist) {
		return false, nil
	}

	log.Errorf("check file %s err: %s", flag, err)
	return false, err
}
