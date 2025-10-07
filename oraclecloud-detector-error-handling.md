# Oracle Cloud Resource Detector – Error Handling Discussion

## 1. Background

In reviewing the initial implementation of the Oracle Cloud resource detector, maintainers provided the following feedback:

> This approach is OK, but you may want to try and differentiate between the case where someone is not on oracle cloud (where return `pcommon.NewResource(), "", nil` is correct), and the case where someone is on oracle cloud, but the request failed (e.g. a timeout?). Currently, it is possible for a user to end up with missing resource attributes in the latter case.

## 2. Current Detector Behavior

- The `Detect()` function attempts to fetch Oracle Cloud instance metadata.
- If fetching metadata fails (for any reason), the detector:
    - Logs the error at debug level.
    - Returns an empty Resource and no error.
- There is no distinction made between:
    - (a) not running on Oracle Cloud, and
    - (b) running on Oracle Cloud with a temporary/partial metadata fetch failure.

## 3. Desired Behavior

Maintainers suggest that the resource detector should:
- Distinguish between "not Oracle Cloud" and "Oracle Cloud, but metadata fetch failed."
- On genuine fetch failure *while on Oracle Cloud*, the detector should signal a problem (e.g., return a non-nil error), rather than silently falling back to an empty Resource.

## 4. Strategies from Other Cloud Providers

### GCP Detector

- **Pattern:** Performs a fast "platform probe" using `metadata.OnGCE()`, which attempts a low-cost HTTP request to the well-known GCP instance metadata service (`http://169.254.169.254`). This detects if the service is present, with a very short timeout, without immediately fetching all metadata.
- **Logic structure:**
    - If "not on GCP" → return empty resource, no error.
    - If "on GCP" but attribute fetch fails → logs and returns partial/empty resource, AND returns error.

### AWS EKS Detector

- **Pattern:** Uses clues specific to EKS before making metadata API calls.
    - Checks for the `KUBERNETES_SERVICE_HOST` environment variable.
    - Probes the Kubernetes API to verify environment (e.g., checks if cluster version string contains `-eks-`).
- Performs network metadata requests *only if* these clues are positive.

## 5. Possible Approaches for Oracle Cloud

- **Environment Clues:** Investigate existence of Oracle-specific environment variables, files, or config sections that can be cheaply checked to determine cloud platform.
    - Examples: agent marker files, OCI config, distinctive DNS, special startup scripts.
- **DNS/Hostname:** Check for Oracle Cloud-specific DNS search domains or hostname patterns.
- **Network Probe:** (Fallback) Use a very short-timeout HTTP probe to the metadata endpoint (as done in GCP/AWS detectors).
- **Decision Logic:**
    - If a clue or probe indicates "not Oracle Cloud" → return empty resource, no error.
    - If a clue or probe indicates "Oracle Cloud" but metadata fetch fails → return error, avoid silent fallback.

## 6. References

- [GCP Detector Implementation](processor/resourcedetectionprocessor/internal/gcp/gcp.go)
- [AWS EKS Detector Implementation](processor/resourcedetectionprocessor/internal/aws/eks/detector.go)
- [Oracle Cloud Detector Implementation](processor/resourcedetectionprocessor/internal/oraclecloud/oraclecloud.go)
- [OpenTelemetry Semantic Conventions](https://github.com/open-telemetry/opentelemetry-specification/tree/main/specification/resource/semantic_conventions)

## Feedback from Tony Choe from Oracle Saas Observability Team:

> I think the Network Probe against the OCI IMDS endpoint is the best.
>     Using the OS-specific conditions may not work since OCI users can use various OS images or bring their own ones.
>     Using the DNS -- similar question. I don't know a reliable option that would work in every case.
