package server

import (
	"context"
	"errors"
	"fmt"
	"git.tdpain.net/codemicro/magicbox/internal/config"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/eko/gocache/lib/v4/store"
	"io"
	"net/http"
	"regexp"
	"strings"
)

const (
	selectorHeader = "X-Magicbox-Resource"
	hitMissHeader  = "X-Magicbox"
)

func (s *server) rootHandler(rw http.ResponseWriter, req *http.Request) error {
	selector := req.Header.Get(selectorHeader)

	if selector == "" {
		rw.WriteHeader(http.StatusBadRequest)
		return nil
	}

	if req.Method != http.MethodGet {
		rw.WriteHeader(http.StatusMethodNotAllowed)
		return nil
	}

	if err := validateSelector(selector); err != nil {
		// This is HTTP 500 because the thinking is that the selector header should be solely controlled by the reverse
		// proxy sitting in front of Magicbox and should be stripped from any incoming requests if it's included.
		rw.WriteHeader(http.StatusInternalServerError)
		_, _ = rw.Write([]byte(err.Error()))
		return nil
	}

	key := "/" + selector + req.URL.Path

	{
		cacheResp, err := s.cache.Get(context.Background(), key)
		if err == nil {
			// TODO: This could probably be optimised somewhat to not actually do any allocations (ie. unmarshal
			//  cachedResp straight into rw)
			cf := new(cachedFile)
			if err := cf.FromBytes(cacheResp); err != nil {
				return fmt.Errorf("unmarshal cached value for %s: %w", key, err)
			}
			rw.Header().Set(hitMissHeader, "hit")
			rw.Header().Set("Content-Type", cf.ContentType)
			_, _ = rw.Write(cf.Body)
			return nil
		}
	}

	var canAttemptToGetIndexHTML bool
	lookupKey := strings.TrimSuffix(key, "/")
	{
		// Only allow if we think we're looking at a directory, ie. not looking at a file/something with a dot in the last bit
		sp := strings.Split(req.URL.Path, "/")
		canAttemptToGetIndexHTML = !strings.Contains(sp[len(sp)-1], ".")
	}

retryGet:
	objResp, err := s.s3Client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(config.Get().S3BucketName),
		Key:    aws.String(lookupKey),
	})

	if err != nil {
		var s3err awserr.Error
		if errors.As(err, &s3err) {
			if s3err.Code() == s3.ErrCodeNoSuchKey {
				if canAttemptToGetIndexHTML {
					canAttemptToGetIndexHTML = false
					lookupKey += "/index.html"
					goto retryGet
				}
				rw.WriteHeader(http.StatusNotFound)
				return nil
			}
		}
		return fmt.Errorf("get object from S3: %w", err)
	}

	defer objResp.Body.Close()

	body, err := io.ReadAll(objResp.Body)
	if err != nil {
		return fmt.Errorf("read S3 object body: %w", err)
	}

	var contentType string
	if objResp.ContentType == nil {
		contentType = "application/octet-stream"
	} else {
		contentType = *objResp.ContentType
	}

	if err := s.cache.Set(context.Background(), key, (&cachedFile{
		Body:        body,
		ContentType: contentType,
	}).Bytes(), store.WithTags([]string{selector})); err != nil {
		return fmt.Errorf("add item to cache: %w", err)
	}

	rw.Header().Set("Content-Type", contentType)

	if objResp.ContentType == nil {
		rw.Header().Set("Content-Type", "application/octet-stream")
	} else {
		rw.Header().Set("Content-Type", *objResp.ContentType)
	}

	rw.Header().Set(hitMissHeader, "miss")
	_, _ = rw.Write(body)
	return nil
}

var selectionValidationRegexp = regexp.MustCompile(`^[a-zA-Z\d\-_+.]+$`)

func validateSelector(selector string) error {
	if !selectionValidationRegexp.MatchString(selector) {
		return errors.New("invalid selector")
	}
	return nil
}

func (s *server) cacheInvalidationHandler(rw http.ResponseWriter, req *http.Request) error {
	selector := req.PathValue("selector")
	if err := validateSelector(selector); err != nil {
		rw.WriteHeader(http.StatusBadRequest)
		_, _ = rw.Write([]byte(err.Error()))
		return nil
	}

	if err := s.cache.Invalidate(context.Background(), store.WithInvalidateTags([]string{selector})); err != nil {
		return fmt.Errorf("invalidate cache: %w", err)
	}

	rw.WriteHeader(http.StatusNoContent)
	return nil
}
