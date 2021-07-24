package aliyun

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"github.com/tiechui1994/tool/util"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
)

const (
	yunpan = "https://api.aliyundrive.com"
)

type Token struct {
	SboxDriveID  string `json:"default_sbox_drive_id"`
	DeviceID     string `json:"device_id"`
	DriveID      string `json:"default_drive_id"`
	UserID       string `json:"user_id"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

func Refresh(refresh string) (token Token, err error) {
	u := yunpan + "/token/refresh"
	body := map[string]string{
		"refresh_token": refresh,
	}

	raw, err := util.POST(u, body, header)
	if err != nil {
		return token, err
	}

	err = json.Unmarshal(raw, &token)
	return token, err
}

const (
	TYPE_FILE   = "file"
	TYPE_FOLDER = "folder"
)

type File struct {
	DeviceID    string `json:"device_id"`
	DomainID    string `json:"domain_id"`
	EncryptMode string `json:"encrypt_mode"`
	FileID      string `json:"file_id"`
	ParentID    string `json:"parent_file_id"`
	Type        string `json:"type"`
	Name        string `json:"name"`

	Size      int    `json:"size"`
	Category  string `json:"category"`
	Hash      string `json:"content_hash"`
	HashName  string `json:"content_hash_name"`
	Url       string `json:"download_url"`
	Thumbnail string `json:"thumbnail"`
	Extension string `json:"file_extension"`
}

func FileList(fileid string, token Token) (list []File, err error) {
	u := yunpan + "/v2/file/list"
	header := map[string]string{
		"accept":        "application/json",
		"authorization": "Bearer " + token.AccessToken,
		"content-type":  "application/json",
	}
	var body struct {
		All                   string `json:"all"`
		DriveID               string `json:"drive_id"`
		Fields                string `json:"fields"`
		OrderBy               string `json:"order_by"`
		OrderDirection        string `json:"order_direction"`
		Limit                 int    `json:"limit"`
		ParentFileID          string `json:"parent_file_id"`
		UrlExpireSec          int    `json:"url_expire_sec"`
		ImageUrlProcess       string `json:"image_url_process"`
		ImageThumbnailProcess string `json:"image_thumbnail_process"`
		VideoThumbnailProcess string `json:"video_thumbnail_process"`
	}

	body.DriveID = token.DriveID
	body.Fields = "*"
	body.OrderBy = "updated_at"
	body.OrderDirection = "DESC"
	body.Limit = 100
	body.ParentFileID = fileid
	body.UrlExpireSec = 1600
	body.ImageUrlProcess = "image/resize,w_1920/format,jpeg"
	body.ImageThumbnailProcess = "image/resize,w_400/format,jpeg"
	body.VideoThumbnailProcess = "video/snapshot,t_0,f_jpg,ar_auto,w_800"
	raw, err := util.POST(u, body, header)
	if err != nil {
		return list, err
	}

	var result struct {
		Items      []File `json:"items"`
		NextMarker string `json:"next_marker"`
	}

	err = json.Unmarshal(raw, &result)
	if err != nil {
		return list, err
	}

	return result.Items, nil
}

func FileDetail(fileid string, token Token) (file File, err error) {
	u := yunpan + "/v2/file/get"
	header := map[string]string{
		"accept":        "application/json",
		"authorization": "Bearer " + token.AccessToken,
		"content-type":  "application/json",
	}
	var body struct {
		DriveID string `json:"drive_id"`
		FileID  string `json:"file_id"`
	}

	body.DriveID = token.DriveID
	body.FileID = fileid
	raw, err := util.POST(u, body, header)
	if err != nil {
		return file, err
	}

	err = json.Unmarshal(raw, &file)
	if err != nil {
		return file, err
	}

	return file, nil
}

type UploadFolderInfo struct {
	DeviceID     string `json:"device_id"`
	DomainID     string `json:"domain_id"`
	FileID       string `json:"file_id"`
	ParentID     string `json:"parent_file_id"`
	Type         string `json:"type"`
	Name         string `json:"file_name"`
	UploadID     string `json:"upload_id"`
	RapidUpload  bool   `json:"rapid_upload"`
	PartInfoList []struct {
		InternalUploadUrl string `json:"internal_upload_url"`
		PartNumber        int    `json:"part_number"`
		UploadUrl         string `json:"upload_url"`
	}
}

const (
	refuse_mode = "refuse"
	rename_mode = "auto_rename"
)

func CalProof(accesstoken string, path string) string {
	// r := md5(accesstoken)[0:16]
	// i := size
	// 开始: r % i
	// 结束: min(开始+8, size)
	// 区间内容进行 Base64 转换
	info, _ := os.Stat(path)
	md := md5.New()
	md.Write([]byte(accesstoken))
	md5sum := hex.EncodeToString(md.Sum(nil))
	r, _ := strconv.ParseUint(md5sum[0:16], 16, 64)
	i := uint64(info.Size())
	o := r % i
	e := uint64(info.Size())
	if o+8 < e {
		e = o + 8
	}
	data := make([]byte, e-o)
	fd, _ := os.Open(path)
	fd.ReadAt(data, int64(o))

	return base64.StdEncoding.EncodeToString(data)
}

func CreateWithFolder(checkmode, name, filetype, fileid string, token Token, appendargs map[string]interface{}, path ...string) (
	upload UploadFolderInfo, err error) {
	u := yunpan + "/adrive/v2/file/createWithFolders"
	header := map[string]string{
		"accept":        "application/json",
		"authorization": "Bearer " + token.AccessToken,
		"content-type":  "application/json",
	}

	body := map[string]interface{}{
		"check_name_mode": checkmode,
		"drive_id":        token.DriveID,
		"name":            name,
		"parent_file_id":  fileid,
		"type":            filetype,
	}

	if appendargs != nil {
		for k, v := range appendargs {
			body[k] = v
		}
	}

	raw, err := util.POST(u, body, header)
	if err != nil {
		// pre_hash match
		if val, ok := err.(util.CodeError); ok && val == http.StatusConflict {
			buf := make([]byte, 10*1024*1024)
			sh := sha1.New()
			fd, err := os.Open(path[0])
			if err != nil {
				return upload, err
			}
			_, err = io.CopyBuffer(sh, fd, buf)
			if err != nil {
				return upload, err
			}

			args := map[string]interface{}{
				"size":              appendargs["size"],
				"part_info_list":    appendargs["part_info_list"],
				"proof_version":     "v1",
				"proof_code":        CalProof(token.AccessToken, path[0]),
				"content_hash_name": "sha1",
				"content_hash":      strings.ToUpper(hex.EncodeToString(sh.Sum(nil))),
			}
			return CreateWithFolder(checkmode, name, filetype, fileid, token, args)
		}

		return upload, err
	}

	err = json.Unmarshal(raw, &upload)
	return upload, err
}

func UploadFile(path, fileid string, token Token) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	fd, err := os.Open(path)
	if err != nil {
		return err
	}

	data := make([]byte, 1024) // 1K, prehash
	fd.Read(data)
	sh := sha1.New()
	sh.Write(data)
	prehash := hex.EncodeToString(sh.Sum(nil))

	m10 := 10 * 1024 * 1024 // 10M
	part := int(info.Size()) / m10
	if int(info.Size())%m10 != 0 {
		part += 1
	}

	var partlist []map[string]int
	for i := 1; i <= part; i++ {
		partlist = append(partlist, map[string]int{"part_number": i})
	}
	args := map[string]interface{}{
		"pre_hash":       prehash,
		"size":           info.Size(),
		"part_info_list": partlist,
	}
	upload, err := CreateWithFolder(rename_mode, info.Name(), TYPE_FILE, fileid, token, args)
	if err != nil {
		return err
	}

	if upload.RapidUpload {
		return nil
	}

	var (
		wg    sync.WaitGroup
		count int
	)

	for _, part := range upload.PartInfoList {
		count += 1
		wg.Add(1)
		go func(u string, part int) {
			defer wg.Done()
			data := make([]byte, m10)
			fd.ReadAt(data, int64((part-1)*m10))
			util.PUT(u, data, nil)
		}(part.UploadUrl, part.PartNumber)
		if count == 5 {
			wg.Wait()
			count = 0
		}
	}

	if count > 0 {
		wg.Wait()
	}

	u := yunpan + "/v2/file/complete"
	header := map[string]string{
		"accept":        "application/json",
		"authorization": "Bearer " + token.AccessToken,
		"content-type":  "application/json",
	}
	body := map[string]string{
		"drive_id":  token.DriveID,
		"file_id":   upload.FileID,
		"upload_id": upload.UploadID,
	}
	_, err = util.POST(u, body, header)
	return err
}

func CreateDirectory(name, fileid string, token Token) (err error) {
	_, err = CreateWithFolder(refuse_mode, name, TYPE_FOLDER, fileid, token, nil)
	return err
}
