package aliyundrive

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/md5"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dustinxie/ecc"
	"github.com/tiechui1994/tool/log"
	"github.com/tiechui1994/tool/util"
	"github.com/tiechui1994/tool/aliyun/singleflight"
)

const (
	yunpan = "https://api.aliyundrive.com"
)

var (
	header = map[string]string{
		"content-type": "application/json",
	}
)

func calProof(accesstoken string, path string) string {
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

type Token struct {
	SboxDriveID  string `json:"default_sbox_drive_id"`
	DeviceID     string `json:"device_id"`
	DriveID      string `json:"default_drive_id"`
	UserID       string `json:"user_id"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

func Refresh(refresh string) (token Token, err error) {
	u := yunpan + "/token/refresh"
	body := map[string]string{
		"refresh_token": refresh,
	}
	header := map[string]string{
		"accept":       "application/json",
		"content-type": "application/json",
	}

	raw, err := util.POST(u, util.WithBody(body), util.WithHeader(header))
	if err != nil {
		return token, err
	}

	err = json.Unmarshal(raw, &token)
	return token, err
}

//=====================================  file  =====================================

const (
	TYPE_FILE   = "file"
	TYPE_FOLDER = "folder"
)

type File struct {
	DriveID     string `json:"drive_id"`
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

type State struct {
	deviceID  string
	signature string
}

var (
	state *State
	once  sync.Once
)

func initState(token Token) {
	once.Do(func() {
		state = &State{
			deviceID: token.DeviceID,
		}
		privateKey, _ := ecdsa.GenerateKey(ecc.P256k1(), rand.Reader)

		secpAppID := "5dde4e1bdf9e4966b387ba58f4b3fdc3"
		singdata := fmt.Sprintf("%s:%s:%s:%d", secpAppID, token.DeviceID, token.UserID, 0)
		hash := sha256.Sum256([]byte(singdata))
		data, _ := ecc.SignBytes(privateKey, hash[:], ecc.RecID|ecc.LowerS)
		state.signature = hex.EncodeToString(data)

		publicKey := privateKey.PublicKey
		pubKey := hex.EncodeToString(elliptic.Marshal(publicKey.Curve, publicKey.X, publicKey.Y))

		u := "https://api.aliyundrive.com/users/v1/users/device/create_session"
		body := fmt.Sprintf(`{"deviceName":"Chrome浏览器","modelName":"Linux网页版","pubKey":"%v"}`, pubKey)
		header := map[string]string{
			"accept":        "application/json",
			"authorization": "Bearer " + token.AccessToken,
			"content-type":  "application/json",
			"X-Canary":      "client=web,app=adrive,version=v4.3.1",
			"x-device-id":   token.DeviceID,
			"X-Signature":   state.signature,
		}
		raw, err := util.POST(u, util.WithBody(body), util.WithHeader(header), util.WithRetry(3))
		if err != nil {
			fmt.Printf("create_session failed: %v \n", err)
			os.Exit(1)
		}
		var result struct {
			Success bool        `json:"success"`
			Message interface{} `json:"message"`
		}
		err = json.Unmarshal(raw, &result)
		if err != nil {
			fmt.Printf("decode result failed: %v \n", err)
			os.Exit(1)
		}
		if !result.Success {
			fmt.Printf("session failed: %v \n", result.Message)
			os.Exit(1)
		}
	})
}

func commonHeader(token Token) map[string]string {
	initState(token)
	return map[string]string{
		"accept":        "application/json",
		"authorization": "Bearer " + token.AccessToken,
		"content-type":  "application/json",
		"X-Canary":      "client=web,app=adrive,version=v4.3.1",
		"x-device-id":   state.deviceID,
		"X-Signature":   state.signature,
	}
}

func Files(parentFileID string, token Token) (list []File, err error) {
	u := yunpan + "/v2/file/list"
	header := commonHeader(token)
	var body struct {
		All                   bool   `json:"all"`
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
	body.ParentFileID = parentFileID
	body.UrlExpireSec = 1600
	body.ImageUrlProcess = "image/resize,w_1920/format,jpeg"
	body.ImageThumbnailProcess = "image/resize,w_400/format,jpeg"
	body.VideoThumbnailProcess = "video/snapshot,t_0,f_jpg,ar_auto,w_800"
	raw, err := util.POST(u,
		util.WithHeader(header),
		util.WithBody(body),
		util.WithRetry(3))
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

func FileInfo(fileid string, token Token) (file File, err error) {
	u := yunpan + "/v2/file/get"
	header := commonHeader(token)
	var body struct {
		DriveID string `json:"drive_id"`
		FileID  string `json:"file_id"`
	}

	body.DriveID = token.DriveID
	body.FileID = fileid
	raw, err := util.POST(u, util.WithBody(body), util.WithHeader(header))
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
	} `json:"part_info_list"`
}

const (
	refuse_mode    = "refuse"
	rename_mode    = "auto_rename"
	overwrite_mode = "overwrite"
)

const (
	m10 = 32 * 1024 * 1024
)

func CreateWithFolder(checkmode, name, filetype, fileid string, token Token, args map[string]interface{}, path ...string) (
	upload UploadFolderInfo, err error) {
	u := yunpan + "/adrive/v2/file/createWithFolders"
	header := commonHeader(token)

	body := map[string]interface{}{
		"check_name_mode": checkmode,
		"drive_id":        token.DriveID,
		"name":            name,
		"parent_file_id":  fileid,
		"type":            filetype,
	}

	if filetype == TYPE_FOLDER {
		raw, err := util.POST(u, util.WithHeader(header), util.WithBody(body))
		if err != nil {
			return upload, err
		}
		err = json.Unmarshal(raw, &upload)
		return upload, err
	}

	directCreate := func() (upload UploadFolderInfo, err error) {
		if len(path) == 0 {
			return upload, errors.New("invliad path")
		}

		buf := make([]byte, m10)
		hash := sha1.New()
		fd, err := os.Open(path[0])
		if err != nil {
			return upload, err
		}
		_, err = io.CopyBuffer(hash, fd, buf)
		if err != nil {
			return upload, err
		}
		sha1sum := strings.ToUpper(hex.EncodeToString(hash.Sum(nil)))

		body = map[string]interface{}{
			"check_name_mode":   checkmode,
			"drive_id":          token.DriveID,
			"name":              name,
			"parent_file_id":    fileid,
			"type":              filetype,
			"size":              args["size"],
			"part_info_list":    args["part_info_list"],
			"proof_version":     "v1",
			"proof_code":        calProof(token.AccessToken, path[0]),
			"content_hash_name": "sha1",
			"content_hash":      sha1sum,
		}

		raw, err := util.POST(u, util.WithBody(body), util.WithHeader(header))
		if err != nil {
			return upload, err
		}
		err = json.Unmarshal(raw, &upload)
		return upload, err
	}

	if filetype == TYPE_FILE && args["size"].(int64) < m10 {
		return directCreate()
	}

	// other
	body["pre_hash"] = args["pre_hash"]
	body["size"] = args["size"]
	body["part_info_list"] = args["part_info_list"]
	raw, err := util.POST(u, util.WithBody(body), util.WithHeader(header))
	if err != nil {
		// pre_hash match
		if val, ok := err.(util.CodeError); ok && val.Code == http.StatusConflict {
			return directCreate()
		}
		return upload, err
	}

	err = json.Unmarshal(raw, &upload)
	return upload, err
}

func UploadFile(path, fileid string, token Token) (id string, err error) {
	info, err := os.Stat(path)
	if err != nil {
		return id, err
	}
	fd, err := os.Open(path)
	if err != nil {
		return id, err
	}

	data := make([]byte, 1024) // 1K, prehash
	fd.Read(data)
	sh := sha1.New()
	sh.Write(data)
	prehash := hex.EncodeToString(sh.Sum(nil))

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
	upload, err := CreateWithFolder(rename_mode, info.Name(), TYPE_FILE, fileid, token, args, path)
	if err != nil {
		return id, err
	}

	log.Infoln("upload file=%q fileid=%v prehash=%v", path, upload.FileID, prehash)
	if upload.RapidUpload {
		log.Infoln("upload file=%q has exist")
		return upload.FileID, nil
	}

	log.Infoln("upload file=%q chunks: %d", path, len(upload.PartInfoList))
	for k := 0; k < len(upload.PartInfoList); k += 1 {
		info := upload.PartInfoList[k]
		log.Infoln("upload file=%q chunk: %d, size: %d", path, info.PartNumber, m10)
		err = uploadFilePart(info.UploadUrl, fd, int64((info.PartNumber-1)*m10), m10)
		if err != nil {
			return upload.FileID, err
		}
	}

	log.Infoln("upload file=%q complete", path)
	u := yunpan + "/v2/file/complete"
	header := commonHeader(token)
	body := map[string]string{
		"drive_id":  token.DriveID,
		"file_id":   upload.FileID,
		"upload_id": upload.UploadID,
	}
	_, err = util.POST(u, util.WithBody(body), util.WithHeader(header), util.WithRetry(3))
	return upload.FileID, err
}

func uploadFilePart(uploadUrl string, file *os.File, start, length int64) error {
	data := make([]byte, length)
	n, _ := file.ReadAt(data, start)

	_, err := util.PUT(uploadUrl, util.WithBody(data[:n]), util.WithRetry(3))
	if err != nil {
		return err
	}

	return nil
}

func CreateDirectory(name, fileid string, token Token) (upload UploadFolderInfo, err error) {
	return CreateWithFolder(refuse_mode, name, TYPE_FOLDER, fileid, token, nil)
}

type batchRequest struct {
	Body    interface{}       `json:"body"`
	Headers map[string]string `json:"headers"`
	ID      string            `json:"id"`
	Method  string            `json:"method"`
	Url     string            `json:"url"`
}

func Batch(requests []batchRequest, token Token) error {
	u := yunpan + "/v2/batch"
	header := commonHeader(token)

	body := map[string]interface{}{
		"requests": requests,
		"resource": "file",
	}
	_, err := util.POST(u, util.WithBody(body), util.WithHeader(header))
	return err
}

func Move(tofileid string, files []File, token Token) error {
	var requests []batchRequest
	for _, file := range files {
		requests = append(requests, batchRequest{
			Url:    "/file/move",
			Method: "POST",
			ID:     file.FileID,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: map[string]interface{}{
				"drive_id":          file.DriveID,
				"file_id":           file.FileID,
				"to_drive_id":       file.DriveID,
				"to_parent_file_id": tofileid,
			},
		})
	}
	return Batch(requests, token)
}

func Trash(files []File, token Token) error {
	var requests []batchRequest
	for _, file := range files {
		requests = append(requests, batchRequest{
			Url:    "/recyclebin/trash",
			Method: "POST",
			ID:     file.FileID,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: map[string]interface{}{
				"drive_id": file.DriveID,
				"file_id":  file.FileID,
			},
		})
	}
	return Batch(requests, token)
}

func Delete(files []File, token Token) error {
	var requests []batchRequest
	for _, file := range files {
		requests = append(requests, batchRequest{
			Url:    "/file/delete",
			Method: "POST",
			ID:     file.FileID,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: map[string]interface{}{
				"drive_id": file.DriveID,
				"file_id":  file.FileID,
			},
		})
	}
	return Batch(requests, token)
}

func Rename(name, fileid string, token Token) error {
	u := yunpan + "/v3/file/update"
	header := commonHeader(token)

	body := map[string]interface{}{
		"check_name_mode": refuse_mode,
		"drive_id":        token.DriveID,
		"file_id":         fileid,
		"name":            name,
	}

	_, err := util.POST(u, util.WithBody(body), util.WithHeader(header))
	return err
}

type DownloadUrl struct {
	Url         string    `json:"url"`
	InternalUrl string    `json:"internal_url"`
	Expiration  time.Time `json:"expiration"`
	Size        int       `json:"size"`
	ContentHash string    `json:"content_hash"`
}

func GetDownloadUrl(file File, token Token) (du DownloadUrl, err error) {
	u := yunpan + "/v2/file/get_download_url"
	body := map[string]interface{}{
		"file_id":  file.FileID,
		"drive_id": file.DriveID,
	}
	header := commonHeader(token)
	raw, err := util.POST(u, util.WithBody(body), util.WithHeader(header))
	if err != nil {
		return du, err
	}

	err = json.Unmarshal(raw, &du)
	return du, err
}

func Download(file File, parallel int, dir string, token Token) error {
	du, err := GetDownloadUrl(file, token)
	if err != nil {
		return err
	}

	if parallel > 10 {
		parallel = 10
	}
	if parallel <= 3 {
		parallel = 3
	}

	m32 := uint(1024 * 1024 * 32)
	batch := uint(du.Size) / m32
	if uint(du.Size)%m32 != 0 {
		batch += 1
	}

	fd, err := os.Create(filepath.Join(dir, file.Name))
	if err != nil {
		return err
	}

	log.Infoln("download file=%q size=%v sha1=%v ", file.Name, file.Size, file.Hash)
	var group singleflight.Group
	var wg sync.WaitGroup
	count := 0
	log.Infoln("download file=%q parallel=%v batch %v", file.Name, parallel, batch)
	for i := uint(0); i < batch; i++ {
		wg.Add(1)
		count += 1
		go func(idx uint) {
			defer wg.Done()
			log.Infoln("download file=%q batch index %v", file.Name, idx)
			retry := 1
		again:
			from := idx * m32
			to := (idx+1)*m32 - 1
			if to >= uint(du.Size) {
				to = uint(du.Size) - 1
			}
			header = map[string]string{
				"connection": "keep-alive",
				"referer":    "https://www.aliyundrive.com/",
				"range":      fmt.Sprintf("bytes=%v-%v", from, to),
			}
			raw, err := util.GET(du.Url, util.WithHeader(header))
			if err != nil {
				if strings.Contains(err.Error(), "Forbidden") && retry <= 3 {
					retry += 1
					val, err, _ := group.Do("url", func() (interface{}, error) {
						log.Errorln("download file=%q batch index %v error: %v", i, err)
						return GetDownloadUrl(file, token)
					})
					if err == nil {
						du = val.(DownloadUrl)
						group.Forget("url")
						goto again
					}
				}
				return
			}
			log.Infoln("download file=%q batch index %v success", file.Name, idx)
			fd.WriteAt(raw, int64(from))
			fd.Sync()
		}(i)
		if count == parallel {
			wg.Wait()
			count = 0
		}
	}

	if count > 0 {
		wg.Wait()
	}
	log.Infoln("download file=%q complete", file.Name)

	return nil
}

//=====================================  share  =====================================

type ShareInfo struct {
	ShareID    string   `json:"share_id"`
	ShareName  string   `json:"share_name"`
	ShareUrl   string   `json:"share_url"`
	Expiration string   `json:"expiration"`
	Status     string   `json:"status"`
	FileIDList []string `json:"file_id_list"`
}

func Share(fileidlist []string, token Token) (share ShareInfo, err error) {
	u := yunpan + "/adrive/v2/share_link/create"
	header := commonHeader(token)
	body := map[string]interface{}{
		"drive_id":     token.DriveID,
		"expiration":   time.Now().Add(7 * time.Hour * 24).Format("2006-01-02T15:04:05.000Z"),
		"file_id_list": fileidlist,
	}

	raw, err := util.POST(u, util.WithBody(body), util.WithHeader(header))
	if err != nil {
		return share, err
	}

	err = json.Unmarshal(raw, &share)
	return share, err
}

func ShareList(token Token) (list []ShareInfo, err error) {
	u := yunpan + "/adrive/v2/share_link/list"
	header := commonHeader(token)
	body := map[string]interface{}{
		"creator":          token.UserID,
		"include_canceled": false,
		"order_by":         "created_at",
		"order_direction":  "DESC",
	}

	raw, err := util.POST(u, util.WithBody(body), util.WithHeader(header))
	if err != nil {
		return list, err
	}

	var result struct {
		Items      []ShareInfo `json:"items"`
		NextMarker string      `json:"next_marker"`
	}

	err = json.Unmarshal(raw, &result)
	return result.Items, err
}

func ShareCancel(shareidlist []string, token Token) error {
	var requests []batchRequest
	for _, shareid := range shareidlist {
		requests = append(requests, batchRequest{
			Url:    "/share_link/cancel",
			Method: "POST",
			ID:     shareid,
			Headers: map[string]string{
				"Content-Type": "application/json",
			},
			Body: map[string]interface{}{
				"share_id": shareid,
			},
		})
	}

	return Batch(requests, token)
}

//=====================================  image  =====================================

func Search(fileid string, token Token) (list []File, err error) {
	u := yunpan + "/v2/file/search"
	header := commonHeader(token)
	var body struct {
		DriveID               string `json:"drive_id"`
		Limit                 int    `json:"limit"`
		ImageUrlProcess       string `json:"image_url_process"`
		ImageThumbnailProcess string `json:"image_thumbnail_process"`
		VideoThumbnailProcess string `json:"video_thumbnail_process"`
		Query                 string `json:"query"`
	}

	body.DriveID = token.DriveID
	body.Limit = 100
	body.ImageUrlProcess = "image/resize,w_1920/format,jpeg"
	body.ImageThumbnailProcess = "image/resize,w_400/format,jpeg"
	body.VideoThumbnailProcess = "video/snapshot,t_0,f_jpg,ar_auto,w_1000"
	body.Query = "type = \"file\""
	raw, err := util.POST(u, util.WithBody(body), util.WithHeader(header))
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

func Create(checkmode, name, filetype, fileid string, token Token, appendargs map[string]interface{}, path ...string) (
	upload UploadFolderInfo, err error) {
	u := yunpan + "/adrive/v1/biz/albums/file/create"
	header := commonHeader(token)

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

	raw, err := util.POST(u, util.WithBody(body), util.WithHeader(header))
	if err != nil {
		// pre_hash match
		if val, ok := err.(util.CodeError); ok && val.Code == http.StatusConflict {
			if len(path) == 0 {
				return upload, errors.New("invliad path")
			}
			sh := sha1.New()
			fd, err := os.Open(path[0])
			if err != nil {
				return upload, err
			}
			_, err = io.CopyBuffer(sh, fd, make([]byte, 10*1024*1024))
			if err != nil {
				return upload, err
			}

			args := map[string]interface{}{
				"size":              appendargs["size"],
				"part_info_list":    appendargs["part_info_list"],
				"proof_version":     "v1",
				"proof_code":        calProof(token.AccessToken, path[0]),
				"content_hash_name": "sha1",
				"content_hash":      strings.ToUpper(hex.EncodeToString(sh.Sum(nil))),
			}
			return Create(checkmode, name, filetype, fileid, token, args)
		}

		return upload, err
	}

	err = json.Unmarshal(raw, &upload)
	return upload, err
}

func UploadImage(path string, token Token) error {
	imageUpload := func(path string) error {
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
		upload, err := Create(rename_mode, info.Name(), TYPE_FILE, "root", token, args)
		if err != nil {
			return err
		}

		if upload.RapidUpload {
			return nil
		}

		for _, part := range upload.PartInfoList {
			info := part
			data := make([]byte, m10)
			fd.ReadAt(data, int64((info.PartNumber-1)*m10))
			util.PUT(info.UploadUrl, util.WithBody(data), util.WithRetry(2))
		}

		return nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	path, err = filepath.Abs(path)
	if err != nil {
		return err
	}

	var files []string
	if info.IsDir() {
		filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
			if !info.IsDir() {
				files = append(files, path)
			}
			return nil
		})
	} else {
		files = append(files, path)
	}

	wg := sync.WaitGroup{}
	count := 0
	for i := 0; i < len(files); i++ {
		path := files[i]
		count += 1
		wg.Add(1)
		go func(path string) {
			defer wg.Done()
			imageUpload(path)
		}(path)

		if count == 5 {
			wg.Wait()
			count = 0
		}
	}

	if count > 0 {
		wg.Wait()
	}

	return nil
}
