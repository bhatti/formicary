package utils

import (
	"fmt"
	"k8s.io/apimachinery/pkg/util/validation"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	// DNS1123NameMaximumLength -- Defines the maximum length of a DNS entry name
	DNS1123NameMaximumLength = 63
	// DNS1123NotAllowedCharacters -- Define characters not allowed in DNS name
	DNS1123NotAllowedCharacters = "[^-a-z0-9]"
	// DNS1123NotAllowedStartCharacters -- Define characters that cannot start DNS name
	DNS1123NotAllowedStartCharacters = "^[^a-z0-9]+"
)

func isEmptyOrWild(str string) bool {
	return str == "" || str == "*"
}

func isWildMatches(str1 string, str2 string) bool {
	return isEmptyOrWild(str1) || isEmptyOrWild(str2) || strings.EqualFold(str1, str2)
}

// SplitTags split tags
func SplitTags(tags string) []string {
	splitRegex := regexp.MustCompile("[, \\t;]")
	arr := splitRegex.Split(tags, -1)
	res := make([]string, 0)
	for _, next := range arr {
		next = strings.TrimSpace(next)
		if next != "" {
			res = append(res, next)
		}
	}
	sort.Slice(res, func(i, j int) bool { return res[i] < res[j] })
	return res
}

// MatchTags match tags
func MatchTags(
	tags string,
	otherTags string) error {
	if !isEmptyOrWild(tags) && !isEmptyOrWild(otherTags) {
		tagArr := SplitTags(tags)
		otherTagArr := SplitTags(otherTags)
		return MatchTagsArray(tagArr, otherTagArr)
	} else if isEmptyOrWild(tags) && !isEmptyOrWild(otherTags) {
		return fmt.Errorf("failed to find any tag for %s", otherTags)
	}
	return nil
}

// MatchTagsArray matches tags
func MatchTagsArray(
	tags []string,
	otherTags []string) error {
	if len(tags) > 0 && len(otherTags) > 0 {
		for _, otherTag := range otherTags {
			if strings.TrimSpace(otherTag) == "" {
				continue
			}
			matches := false
			for _, tag := range tags {
				if strings.EqualFold(tag, otherTag) {
					matches = true
					break
				}
			}
			if !matches {
				return fmt.Errorf("failed to find %s in %s", otherTag, tags)
			}
		}
	} else if len(tags) == 0 && len(otherTags) > 0 {
		return fmt.Errorf("retry.gofailed to find any tag for %v", otherTags)
	}
	return nil
}

// MakeDNS1123Compatible removes special or invalid characters because kubernetes doesn't allow it
func MakeDNS1123Compatible(name string) string {
	name = strings.ToLower(name)
	name = strings.Replace(name, "_", "-", -1)
	//
	nameNotAllowedChars := regexp.MustCompile(DNS1123NotAllowedCharacters)
	name = nameNotAllowedChars.ReplaceAllString(name, "")

	nameNotAllowedStartChars := regexp.MustCompile(DNS1123NotAllowedStartCharacters)
	name = nameNotAllowedStartChars.ReplaceAllString(name, "")

	if len(name) > DNS1123NameMaximumLength {
		name = name[0:DNS1123NameMaximumLength]
	}

	return name
}

// ReplaceDirPath joins path with dir path
func ReplaceDirPath(paths []string, dir string) (res []string) {
	if dir == "" {
		return paths
	}
	res = make([]string, len(paths))
	for i, k := range paths {
		res[i] = filepath.Join(dir, filepath.Base(k))
	}
	return
}

// RFC1123SubdomainError error
type RFC1123SubdomainError struct {
	errs []string
}

const emptyRFC1123SubdomainErrorMessage = "validating rfc1123 subdomain"

func (d *RFC1123SubdomainError) Error() string {
	if len(d.errs) == 0 {
		return emptyRFC1123SubdomainErrorMessage
	}

	return strings.Join(d.errs, ", ")
}

// ValidateDNS1123Subdomain validation
func ValidateDNS1123Subdomain(name string) error {
	errs := validation.IsDNS1123Subdomain(name)
	if len(errs) == 0 {
		return nil
	}

	return &RFC1123SubdomainError{errs: errs}
}
