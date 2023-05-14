package speech

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tiechui1994/tool/util"
)

func randomBoundary() string {
	var buf [8]byte
	_, err := io.ReadFull(rand.Reader, buf[:])
	if err != nil {
		panic(err)
	}
	return "------------------------" + hex.EncodeToString(buf[:])
}

var quoteEscaper = strings.NewReplacer("\\", "\\\\", `"`, "\\\"")

func escapeQuotes(s string) string {
	return quoteEscaper.Replace(s)
}

func uploadFile(fields map[string]string) (reader io.Reader, contentType string, totalSize int) {
	boundary := randomBoundary()
	contentType = fmt.Sprintf("multipart/form-data; boundary=%s", boundary)

	parts := make([]io.Reader, 0)
	CRLF := "\r\n"

	fieldBoundary := "--" + boundary + CRLF

	for k, v := range fields {
		parts = append(parts, strings.NewReader(fieldBoundary))
		totalSize += len(fieldBoundary)

		header := fmt.Sprintf(`Content-Disposition: form-data; name="%s"`, escapeQuotes(k))
		parts = append(
			parts,
			strings.NewReader(header+CRLF+CRLF),
			strings.NewReader(v),
			strings.NewReader(CRLF),
		)
		totalSize += len(header) + 2*len(CRLF) + len(v) + len(CRLF)
	}

	finishBoundary := "--" + boundary + "--" + CRLF
	parts = append(parts, strings.NewReader(finishBoundary))
	totalSize += len(finishBoundary)

	return io.MultiReader(parts...), contentType, totalSize
}

func HMACSha1(key, method, md5, _type, date string, ossHeader []string, resource string) string {
	values := []string{
		method, md5, _type, date,
	}

	sort.Slice(ossHeader, func(i, j int) bool {
		return ossHeader[i] < ossHeader[j]
	})

	values = append(values, ossHeader...)
	values = append(values, resource)

	mac := hmac.New(sha1.New, []byte(key))
	mac.Write([]byte(strings.Join(values, "\n")))
	res := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return res
}

func SpeechToText(src string) (result string, err error) {
	fd, err := os.Open(src)
	if err != nil {
		return result, err
	}

	stat, _ := fd.Stat()

	_, name := filepath.Split(fd.Name())
	reader, contentType, totalSize := uploadFile(map[string]string{
		"task_type":   "201",
		"filenames[]": name,
	})
	header := map[string]string{
		"content-type":   contentType,
		"content-length": fmt.Sprintf("%v", totalSize),
		"origin":         "https://reccloud.cn",
		"x-api-key":      "wxonf9nu5elogwxzp",
	}

	u := "https://aw.aoscdn.com/tech/authorizations/oss"
	raw, err := util.POST(u, util.WithBody(reader), util.WithHeader(header), util.WithRetry(3))
	if err != nil {
		return result, err
	}

	var authorizate struct {
		Status int `json:"status"`
		Data   struct {
			Accelerate string `json:"accelerate"`
			Bucket     string `json:"bucket"`
			Region     string `json:"region"`
			Endpoint   string `json:"endpoint"`
			Callback   struct {
				Url  string `json:"url"`
				Type string `json:"type"`
				Body string `json:"body"`
			} `json:"callback"`
			Credential struct {
				AccessKeyID     string `json:"access_key_id"`
				AccessKeySecret string `json:"access_key_secret"`
				Expiration      int    `json:"expiration"`
				SecurityToken   string `json:"security_token"`
			} `json:"credential"`
			Objects map[string]string `json:"objects"`
		} `json:"data"`
	}
	err = json.Unmarshal(raw, &authorizate)
	if err != nil {
		return result, err
	}

	type UploadPartResult struct {
		PartNumber int    `xml:"PartNumber"`
		ETag       string `xml:"ETag"`
	}

	const Size = 128000 * 2
	buffer := make([]byte, Size)
	for _, task := range authorizate.Data.Objects {
		endpoint := fmt.Sprintf("https://%v.%v/%v", authorizate.Data.Bucket, authorizate.Data.Endpoint, task)
		date := time.Now().In(time.UTC).Format("Mon, 02 Jan 2006 15:04:05 GMT")
		ossHeader := []string{
			"x-oss-date:" + date,
			"x-oss-security-token:" + authorizate.Data.Credential.SecurityToken,
			"x-oss-user-agent:aliyun-sdk-js/6.17.1 Chrome 112.0.0.0 on Windows 10 64-bit",
		}
		uri := "?uploads"
		header := map[string]string{
			"origin":               "https://reccloud.cn",
			"x-oss-date":           date,
			"x-oss-security-token": authorizate.Data.Credential.SecurityToken,
			"x-oss-user-agent":     "aliyun-sdk-js/6.17.1 Chrome 112.0.0.0 on Windows 10 64-bit",
		}
		header["authorization"] = "OSS " + authorizate.Data.Credential.AccessKeyID + ":" +
			HMACSha1(authorizate.Data.Credential.AccessKeySecret, "POST", "",
				"", date, ossHeader, "/"+authorizate.Data.Bucket+"/"+task+uri)
		raw, err = util.POST(endpoint+uri, util.WithBody(reader), util.WithHeader(header), util.WithRetry(3))
		if err != nil {
			return result, err
		}
		var uploads struct {
			XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
			UploadId string   `xml:"UploadId"`
			Key      string   `xml:"Key"`
			Bucket   string   `xml:"Bucket"`
		}

		err = xml.Unmarshal(raw, &uploads)
		if err != nil {
			return result, err
		}

		// uploading
		var uploadResult struct {
			XMLName xml.Name           `xml:"CompleteMultipartUpload"`
			Parts   []UploadPartResult `xml:"Part"`
		}

		parts := stat.Size() / Size
		if stat.Size()%Size != 0 {
			parts += 1
		}
		uploadResult.Parts = make([]UploadPartResult, parts)
		for partNo := 1; partNo <= int(parts); partNo++ {
			length, err := fd.ReadAt(buffer, int64(partNo-1)*Size)
			if err != nil && err != io.EOF {
				return result, err
			}

			uri := fmt.Sprintf("?partNumber=%v&uploadId=%v", partNo, uploads.UploadId)
			header["authorization"] = "OSS " + authorizate.Data.Credential.AccessKeyID + ":" +
				HMACSha1(authorizate.Data.Credential.AccessKeySecret,
					"PUT", "", "", date,
					ossHeader, "/"+authorizate.Data.Bucket+"/"+task+uri)
			_, responseHeader, err := util.Request("PUT", endpoint+uri, util.WithBody(buffer[:length]), util.WithHeader(header), util.WithRetry(3))
			if err != nil {
				return result, err
			}

			uploadResult.Parts[partNo-1].PartNumber = partNo
			uploadResult.Parts[partNo-1].ETag = responseHeader.Get("ETag")
		}

		// finish
		raw, err = xml.Marshal(uploadResult)
		if err != nil {
			return
		}
		body := []byte(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
		body = append(body, raw...)
		md5 := base64.StdEncoding.EncodeToString(util.MD5(string(body)))

		callBody := strings.ReplaceAll(authorizate.Data.Callback.Body, "${filename}", name)
		callback := fmt.Sprintf(`{"callbackUrl":"%v","callbackBody":"%v"}`,
			authorizate.Data.Callback.Url, callBody)
		uri = fmt.Sprintf("?uploadId=%v", uploads.UploadId)

		ossHeader = append(ossHeader, "x-oss-callback:"+base64.StdEncoding.EncodeToString([]byte(callback)))
		header["x-oss-callback"] = base64.StdEncoding.EncodeToString([]byte(callback))
		header["content-md5"] = md5
		header["content-type"] = "application/xml"
		header["authorization"] = "OSS " + authorizate.Data.Credential.AccessKeyID + ":" +
			HMACSha1(authorizate.Data.Credential.AccessKeySecret,
				"POST", md5, "application/xml", date,
				ossHeader, "/"+authorizate.Data.Bucket+"/"+task+uri)
		raw, err = util.POST(endpoint+uri, util.WithBody(body), util.WithHeader(header), util.WithRetry(3))
		if err != nil {
			return result, err
		}

		var complete struct {
			Status int `json:"status"`
			Data   struct {
				Url        string `json:"url"`
				ResourceID string `json:"resource_id"`
			} `json:"data"`
		}
		err = json.Unmarshal(raw, &complete)
		if err != nil {
			return result, err
		}
		delete(header, "content-md5")

		return recognition(complete.Data.ResourceID, name)
	}

	return result, fmt.Errorf("invalid task")
}

func recognition(resourceID, filename string) (result string, err error) {
	reader, contentType, totalSize := uploadFile(map[string]string{
		"language":     "",
		"return_type":  "1",
		"type":         "4",
		"content_type": "1",
		"resource_id":  resourceID,
		"filename":     filename,
	})
	header := map[string]string{
		"content-type":   contentType,
		"content-length": fmt.Sprintf("%v", totalSize),
		"origin":         "https://reccloud.cn",
		"x-api-key":      "wx40d7754m8oubrds",
	}
	u := "https://aw.aoscdn.com/tech/tasks/audio/recognition"
	raw, err := util.POST(u, util.WithBody(reader), util.WithHeader(header), util.WithRetry(3))
	if err != nil {
		return result, err
	}

	var recognition struct {
		Status int `json:"status"`
		Data   struct {
			TaskID string `json:"task_id"`
		} `json:"data"`
	}
	err = json.Unmarshal(raw, &recognition)
	if err != nil {
		return result, err
	}

	u = "https://aw.aoscdn.com/tech/tasks/audio/recognition/" + recognition.Data.TaskID
	header = map[string]string{
		"origin":    "https://reccloud.cn",
		"x-api-key": "wx40d7754m8oubrds",
	}
again:
	raw, err = util.GET(u, util.WithHeader(header), util.WithRetry(2))
	if err != nil {
		time.Sleep(2 * time.Second)
		goto again
	}
	var status struct {
		Status int `json:"status"`
		Data   struct {
			Progress int    `json:"progress"`
			State    int    `json:"state"`
			Result   string `json:"result"`
			File     string `json:"file"`
		} `json:"data"`
	}
	err = json.Unmarshal(raw, &status)
	if err != nil {
		time.Sleep(2 * time.Second)
		goto again
	}
	if status.Data.Progress < 100 {
		time.Sleep(time.Second)
		goto again
	}

	return status.Data.Result, nil
}
