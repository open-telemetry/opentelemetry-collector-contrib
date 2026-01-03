package awsecsattributesprocessor

import (
	"context"
	"fmt"
	"regexp"

	"github.com/open-telemetry/opentelemetry-collector-contrib/processor/awsecsattributesprocessor/internal/metadata"
	"go.opentelemetry.io/collector/consumer/xconsumer"
	"go.opentelemetry.io/collector/pdata/pprofile"
	"go.uber.org/zap"
)

var (
	profileIdReg = regexp.MustCompile(`[a-z0-9]{64}`)
)

// profilesDataProcessor implements DataProcessor for profiles
type profilesDataProcessor struct {
	next xconsumer.Profiles
}

var _ DataProcessor[pprofile.Profiles] = (*profilesDataProcessor)(nil)

func (pdp *profilesDataProcessor) Process(ctx context.Context, logger *zap.Logger, cfg *Config, getMeta MetadataGetter, pd pprofile.Profiles) error {
	for i := 0; i < pd.ResourceProfiles().Len(); i++ {
		rprofile := pd.ResourceProfiles().At(i)
		containerID := getProfileContainerId(&rprofile, cfg.ContainerID.Sources...)
		logger.Debug("processing",
			zap.String("container.id", containerID),
			zap.String("processor", metadata.Type.String()))

		meta, err := getMeta(containerID)
		if err != nil {
			logger.Debug("unable to retrieve metadata",
				zap.String("container.id", containerID),
				zap.String("processor", metadata.Type.String()),
				zap.Error(err))

			continue
		}

		// flatten the data
		flattened := meta.Flat()

		for k, v := range flattened {
			val := fmt.Sprintf("%v", v)
			// if not allowed or empty value, skip
			if !cfg.allowAttr(k) || val == "" {
				continue
			}

			rprofile.Resource().Attributes().
				PutStr(k, val)
		}
	}
	return nil
}

func (pdp *profilesDataProcessor) Forward(ctx context.Context, pd pprofile.Profiles) error {
	return pdp.next.ConsumeProfiles(ctx, pd)
}

// profilesProcessor wraps processorBase to implement xprocessor.Profiles interface
type profilesProcessor struct {
	*processorBase[pprofile.Profiles, *profilesDataProcessor]
}

func (pp *profilesProcessor) ConsumeProfiles(ctx context.Context, pd pprofile.Profiles) error {
	return pp.Consume(ctx, pd)
}

// setNextConsumer sets the next consumer (used for testing)
func (pp *profilesProcessor) setNextConsumer(next xconsumer.Profiles) {
	pp.dataProcessor.next = next
}

func newProfilesProcessor(
	ctx context.Context,
	logger *zap.Logger,
	cfg *Config,
	next xconsumer.Profiles,
	endpoints endpointsFn,
	containerdata containerDataFn,
) *profilesProcessor {
	dataProcessor := &profilesDataProcessor{next: next}
	base := newProcessorBase(ctx, logger, cfg, dataProcessor, endpoints, containerdata)
	return &profilesProcessor{processorBase: base}
}

func getProfileContainerId(rprofile *pprofile.ResourceProfiles, sources ...string) string {
	var id string
	for _, s := range sources {
		if v, ok := rprofile.Resource().Attributes().Get(s); ok {
			id = v.AsString()
			break
		}
	}

	// strip any unneeded values for eg. file extension
	id = profileIdReg.FindString(id)
	return id
}
