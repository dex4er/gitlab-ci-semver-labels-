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
	log.Printf("[TRACE] BumpPrerelease(version=%v)\n", version)

	newVer, err := ver.SetPrerelease(incrementNumberAsString(ver.Prerelease()))
	if err != nil {
		return "", fmt.Errorf("cannot bump semver: %w", err)
	}

	return newVer.String(), nil
}

func BumpPatch(version string) (string, error) {
	ver, err := semver.NewVersion(version)
	if err != nil {
		return "", err
	}
	log.Printf("[TRACE] BumpPatch(version=%v)\n", version)

	newVer := ver.IncPatch()

	return newVer.String(), nil
}

func BumpMinor(version string) (string, error) {
	ver, err := semver.NewVersion(version)
	if err != nil {
		return "", err
	}
	log.Printf("[TRACE] BumpMinor(version=%v)\n", version)

	newVer := ver.IncMinor()

	return newVer.String(), nil
}

func BumpMajor(version string) (string, error) {
	ver, err := semver.NewVersion(version)
	if err != nil {
		return "", err
	}
	log.Printf("[TRACE] BumpMajor(version=%v)\n", version)

	newVer := ver.IncMajor()

	return newVer.String(), nil
}
