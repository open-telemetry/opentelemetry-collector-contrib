# Agent Upgrades

The Supervisor will download Collector package updates when offered so
by the Backend. The Supervisor will verify the integrity of the packages
and will install them. This requires stopping the Collector, overwriting
the Collector executable file with the newly downloaded version and
starting the Collector. Before overwriting the executable the Supervisor
will save it in case it is necessary for reverting.

## Expected PackagesAvailable Format

The PackagesAvailable message is expected to contain a single 
package. 
* This package MUST have an empty name. 
* The type of the package MUST be top level. 
* The version field SHOULD be the version of the agent with a `v` prefix.
(e.g. `v0.110.0`)
* The download URL MUST point to a gzipped tarball, containing a file named
`otelcol-contrib`. This file will be used as the agent binary.
* The content hash MUST be the sha256 sum of the gzipped tarball.
* The signature field MUST be formatted in the way described
in the [Signature Format](#signature-format) session.

Example:

```go
&protobufs.PackagesAvailable{
    Packages: map[string]*protobufs.PackageAvailable{
        "": {
            Type:    protobufs.PackageType_PackageType_TopLevel,
            Version: "v0.110.0",
            Hash: packageHash
            File: &protobufs.DownloadableFile{
                DownloadUrl: agentURL,
                ContentHash: agentSHA256Hash,
                Signature:   signatureField,
            },
        },
    },
    AllPackagesHash: allPackagesHash,
}
```


## Signing

## Signature Format
