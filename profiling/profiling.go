// Package profiling
package profiling

import (
	"context"

	"github.com/grafana/pyroscope-go"
)

type Config struct {
	Name      string            `yaml:"name"      env:"name, overwrite"`
	Endpoint  string            `yaml:"endpoint"  env:"ENDPOINT, overwrite"`
	Tags      map[string]string `yaml:"tags"`
	Profilers Profilers         `yaml:"profilers" env:", prefix=PROFILERS_"`
	Logger    pyroscope.Logger
}

type Profilers struct {
	CPU           bool `yaml:"cpu"            env:"CPU, default=true"`
	AllocObjects  bool `yaml:"alloc_objects"  env:"ALLOC_OBJECTS, default=true"`
	AllocSpace    bool `yaml:"alloc_space"    env:"ALLOC_SPACE, default=true"`
	InuseObjects  bool `yaml:"inuse_objects"  env:"INUSE_OBJECTS"`
	InuseSpace    bool `yaml:"inuse_space"    env:"INUSE_SPACE"`
	Goroutines    bool `yaml:"goroutines"     env:"GOROUTINES, default=true"`
	BlockCount    bool `yaml:"block_count"    env:"BLOCK_COUNT"`
	BlockDuration bool `yaml:"block_duration" env:"BLOCK_DURATION"`
	MutexCount    bool `yaml:"mutex_count"    env:"MUTEX_COUNT"`
	MutexDuration bool `yaml:"mutex_duration" env:"MUTEX_DURATION"`
}

func (p Profilers) PyroscopeTypes() []pyroscope.ProfileType {
	out := []pyroscope.ProfileType{}
	if p.CPU {
		out = append(out, pyroscope.ProfileCPU)
	}
	if p.AllocObjects {
		out = append(out, pyroscope.ProfileAllocObjects)
	}
	if p.AllocSpace {
		out = append(out, pyroscope.ProfileInuseSpace)
	}
	if p.InuseObjects {
		out = append(out, pyroscope.ProfileInuseObjects)
	}
	if p.InuseSpace {
		out = append(out, pyroscope.ProfileInuseSpace)
	}
	if p.Goroutines {
		out = append(out, pyroscope.ProfileGoroutines)
	}
	if p.BlockCount {
		out = append(out, pyroscope.ProfileBlockCount)
	}
	if p.BlockDuration {
		out = append(out, pyroscope.ProfileBlockDuration)
	}
	if p.MutexCount {
		out = append(out, pyroscope.ProfileMutexCount)
	}
	if p.MutexDuration {
		out = append(out, pyroscope.ProfileMutexDuration)
	}
	return out
}

func Start(conf Config) (context.CancelFunc, error) {
	prof, err := pyroscope.Start(pyroscope.Config{
		ApplicationName: conf.Name,
		ServerAddress:   conf.Endpoint,
		Logger:          conf.Logger,
		Tags:            conf.Tags,
		ProfileTypes:    conf.Profilers.PyroscopeTypes(),
	})
	return func() {
		_ = prof.Stop()
	}, err
}
