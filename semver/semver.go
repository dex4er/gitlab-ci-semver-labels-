package semver

import (
	"fmt"
	"log"
	"strconv"

	"github.com/Masterminds/semver/v3"
)

func IsValid(version string) bool {
	_, err := semver.NewVersion(version)
	return err == nil
}

func incrementNumberAsString(number string) string {
	num, err := strconv.Atoi(number)
	if err != nil {
		num = 0
	}
	result := strconv.Itoa(num + 1)
	return result
}

func BumpPrerelease(version string) (string, error) {
	ver, err := semver.NewVersion(version)
	if err != nil {
		return "", err
	}
	log.Printf("[TRACE] BumpPrerelease(version=%v)", version)

	newVer, err := ver.SetPrerelease(incrementNumberAsString(ver.Prerelease()))
	if err != nil {
		return "", fmt.Errorf("cannot bump semver: %w", err)
	}

	return newVer.String(), nil
}

func BumpPatch(version string, prerelease bool) (string, error) {
	ver, err := semver.NewVersion(version)
	if err != nil {
		return "", err
	}
	log.Printf("[TRACE] BumpPatch(version=%v)", version)

	incVer := ver.IncPatch()

	if !prerelease {
		return incVer.String(), nil
	}

	newVer, err := incVer.SetPrerelease(incrementNumberAsString(ver.Prerelease()))

	if err != nil {
		return "", fmt.Errorf("cannot bump semver: %w", err)
	}

	return newVer.String(), nil
}

func BumpMinor(version string, prerelease bool) (string, error) {
	ver, err := semver.NewVersion(version)
	if err != nil {
		return "", err
	}
	log.Printf("[TRACE] BumpMinor(version=%v)", version)

	incVer := ver.IncMinor()

	if !prerelease {
		return incVer.String(), nil
	}

	newVer, err := incVer.SetPrerelease(incrementNumberAsString(ver.Prerelease()))

	if err != nil {
		return "", fmt.Errorf("cannot bump semver: %w", err)
	}

	return newVer.String(), nil
}

func BumpMajor(version string, prerelease bool) (string, error) {
	ver, err := semver.NewVersion(version)
	if err != nil {
		return "", err
	}
	log.Printf("[TRACE] BumpMajor(version=%v)", version)

	incVer := ver.IncMajor()

	if !prerelease {
		return incVer.String(), nil
	}

	newVer, err := incVer.SetPrerelease(incrementNumberAsString(ver.Prerelease()))

	if err != nil {
		return "", fmt.Errorf("cannot bump semver: %w", err)
	}

	return newVer.String(), nil
}

func Current(version string) (string, error) {
	ver, err := semver.NewVersion(version)
	if err != nil {
		return "", err
	}
	log.Printf("[TRACE] Current(version=%v)", version)

	return ver.String(), nil
}
