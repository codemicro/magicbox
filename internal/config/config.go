package config

import (
	"errors"
	"os"
	"strconv"
	"sync"
)

type Config struct {
	ServerHTTPAddress string
	MaxCacheMB        int

	AdminEnabled      bool
	adminEnabledValid bool
	AdminHTTPAddress  string
	AdminToken        string

	S3BucketName          string
	S3CredentialID        string
	S3CredentialSecret    string
	S3Endpoint            string
	S3Region              string
	S3ForcePathStyle      bool
	s3ForcePathStyleValid bool
}

var (
	confOnce = new(sync.Once)
	conf     *Config
)

func Get() *Config {
	confOnce.Do(func() {
		conf = &Config{
			AdminToken: os.Getenv("MAGICBOX_ADMIN_TOKEN"),

			S3BucketName:       os.Getenv("MAGICBOX_S3_BUCKET_NAME"),
			S3CredentialID:     os.Getenv("MAGICBOX_S3_CREDENTIAL_ID"),
			S3CredentialSecret: os.Getenv("MAGICBOX_S3_CREDENTIAL_SECRET"),
			S3Endpoint:         os.Getenv("MAGICBOX_S3_ENDPOINT"),
			S3Region:           os.Getenv("MAGICBOX_S3_REGION"),
		}

		if v := os.Getenv("MAGICBOX_HTTP_ADDRESS"); v == "" {
			conf.ServerHTTPAddress = "127.0.0.1:8080"
		} else {
			conf.ServerHTTPAddress = v
		}

		if v := os.Getenv("MAGICBOX_MAX_CACHE_MB"); v == "" {
			conf.MaxCacheMB = 1024 // 1GB max
		} else {
			parsedValue, err := strconv.Atoi(v)
			if err != nil {
				parsedValue = -1
			}
			conf.MaxCacheMB = parsedValue
		}

		if v := os.Getenv("MAGICBOX_ADMIN_HTTP_ADDRESS"); v == "" {
			conf.AdminHTTPAddress = "127.0.0.1:8081"
		} else {
			conf.AdminHTTPAddress = v
		}

		conf.adminEnabledValid = true
		if v := os.Getenv("MAGICBOX_ADMIN_ENABLED"); v != "" {
			parsedValue, err := strconv.ParseBool(v)
			if err != nil {
				conf.adminEnabledValid = false
			} else {
				conf.AdminEnabled = parsedValue
			}
		} else {
			conf.AdminEnabled = true
		}

		conf.s3ForcePathStyleValid = true
		if v := os.Getenv("MAGICBOX_S3_FORCE_PATH_STYLE"); v != "" {
			parsedValue, err := strconv.ParseBool(v)
			if err != nil {
				conf.s3ForcePathStyleValid = false
			} else {
				conf.S3ForcePathStyle = parsedValue
			}
		}
	})
	return conf
}

func Validate() error {
	conf := Get()

	if conf.MaxCacheMB == -1 {
		return errors.New("MAGICBOX_MAX_CACHE_MB not an integer")
	}

	if !conf.adminEnabledValid {
		return errors.New("MAGICBOX_ADMIN_ENABLED not a valid boolean")
	}

	if !conf.s3ForcePathStyleValid {
		return errors.New("MAGICBOX_S3_FORCE_PATH_STYLE not a valid boolean")
	}

	if conf.S3BucketName == "" {
		return errors.New("missing required environment variable MAGICBOX_S3_BUCKET_NAME")
	}
	if conf.S3CredentialID == "" {
		return errors.New("missing required environment variable MAGICBOX_S3_CREDENTIAL_ID")
	}
	if conf.S3CredentialSecret == "" {
		return errors.New("missing required environment variable MAGICBOX_S3_CREDENTIAL_SECRET")
	}
	if conf.S3Endpoint == "" {
		return errors.New("missing required environment variable MAGICBOX_S3_ENDPOINT")
	}
	if conf.S3Region == "" {
		return errors.New("missing required environment variable MAGICBOX_S3_REGION")
	}

	return nil
}
