package manager

import "fmt"

// protectConfigStorage migrates legacy permissions under the instance lock.
// Private parent directories also protect backups created by older releases.
func (p *configPipeline) protectConfigStorage() error {
	if err := p.fs.MkdirAll(stateDir, dirPermPrivate); err != nil {
		return fmt.Errorf("creating private state directory: %w", err)
	}
	if err := p.fs.Chmod(stateDir, dirPermPrivate); err != nil {
		return fmt.Errorf("protecting state directory: %w", err)
	}
	for _, path := range []string{configDir, subscriptionDataFile, subscriptionURLFile, subscriptionSourceFile, configApplyStatusFile, configApplyTransactionFile, OverrideFilePath, configYAML, legacyTemplatePath} {
		exists, err := p.fs.FileExists(path)
		if err != nil {
			return fmt.Errorf("checking private path %s: %w", path, err)
		}
		if !exists {
			continue
		}
		mode := uint32(filePermPrivateRW)
		if path == configDir {
			mode = dirPermPrivate
		}
		if err := p.fs.Chmod(path, mode); err != nil {
			return fmt.Errorf("protecting %s: %w", path, err)
		}
	}
	return nil
}
