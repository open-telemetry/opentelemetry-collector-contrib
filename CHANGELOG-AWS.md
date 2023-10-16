# [](https://github.com/bhanuba/opentelemetry-collector-contrib/compare/v2.0.1...v) (2023-10-16)



# Changelog

All notable changes to this project will be documented in this file. See [standard-version](https://github.com/conventional-changelog/standard-version) for commit guidelines.

### [2.0.1](https://github.com/bhanuba/opentelemetry-collector-contrib/compare/v2.0.0...v2.0.1) (2023-10-16)

## 2.0.0 (2023-10-16)


### ⚠ BREAKING CHANGES

* **mysqlreceiver:** removing `mysql.locked_connects` metric which is replaced by `mysql.connection.errors` (#23990)
* changelog entry added
* **recombine:** The default operator ID for `recombine` operator
is now `recombine`, not `metadata`.
* for current users upon upgrading, unless
explicitly setting the `nonalphanumeric_dimension_chars` config
option on the signalfx exporter.

This PR changes the config option `nonalphanumeric_dimension_chars`
default from `_-` to `_-.`.

The resulting behavior change is dimensions keys with periods in them
will no longer be converted to underscores. For example, if the
dimension `k8s.pod.uid` was currently sent, the exporter would
automatically convert this to `k8s_pod-uid`. This change will allow
`k8s.pod.uid` to be sent as is.

Why? The SignalFx backend now allows periods in dimension keys,
and much of our content will rely on this.
Existing users who do not wish to break MTSs and keep current
functionality can set `nonalphanumeric_dimension_chars` to `_-`.
* Rename kinesis exporter to awskinesis
* replaces `service_url` config field for updated `endpoint`

### Features

* add boolean behavior for durations ([#24272](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/24272)) ([6ba05da](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/6ba05da26415a4180368cd950058a1f255d3ee59))
* add boolean behavior for time objects ([#24243](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/24243)) ([983815c](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/983815c6abe77c6e7c23da5035185da86c715790)), closes [#22008](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/22008)
* add datadog trace translation helpers and tests ([#1208](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/1208)) ([8d6f04c](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/8d6f04c2bd4a433ac7f33075dd9a2a06eb78809c))
* add duration to int float converters ([#25069](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/25069)) ([0ad6de0](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/0ad6de0c306dfc4e4cf823127d8426d512caf4c1)), closes [#24686](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/24686)
* Add Sentry Exporter ([#565](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/565)) ([925787d](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/925787d04e552ab3911f7d8539f09d04695b1e4c))
* add time to int converters ([#25147](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/25147)) ([3a6f547](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/3a6f54773c1c494bb95e31307c2faefa6bd38403))
* added support for AWS AppRunner origin ([#6141](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/6141)) ([14a6ff3](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/14a6ff328b06c19a69c71432967b5a8555228dd7))
* allow custom port for cassandra connection ([#25179](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/25179)) ([2ec4c67](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/2ec4c67fe303a85e0624205c78b180ec3e0eb160))
* allow math expressions for time and duration ([#24453](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/24453)) ([905674a](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/905674ad18b71ea7ea1122277ebcb4f713a695b7)), closes [#22009](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/22009) [#22009](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/22009)
* **coreinternal/attraction:** supports pattern for delete and hash attractions ([#10822](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/10822)) ([9a4ab07](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/9a4ab07e8a6f4a7f755ea57b02bd5b5d58490db9))
* delta to cumulative prometheus ([#9919](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/9919)) ([c522cef](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/c522cef4ae42262ac8f7e5922d0729f615b3da07))
* **dynatraceexporter:** provide better estimated summaries for partial histograms ([#11044](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/11044)) ([ccfee86](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/ccfee869a2951ed3f8dbb10c292f01421bfdc6b3))
* **dynatrace:** handle non-gauge data types ([#5056](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/5056)) ([39991a1](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/39991a1252d81c5382915debf18cc2c73e2c7ab5))
* **dynatrace:** serialize metrics using dt metrics utils ([#4908](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/4908)) ([827de29](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/827de29ea56dbcd1b84cedfe82c7d4fcc3291770))
* **dynatrace:** tags is deprecated in favor of default_dimensions ([#5055](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/5055)) ([f77bacd](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/f77bacd11fd42c316b543ed7e98c021c2b14d1bf))
* **fluentforwardreceiver:** support more complex fluent-bit objects ([#5676](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/5676)) ([1ac25f9](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/1ac25f9e94c34c3a708a17e8d4d0af3a65133521))
* improve clickhouse connection management ([#18085](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/18085)) ([954b49f](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/954b49fc0caa0acbd499c7a0c50f8d66ec2fa642))
* **influxdbexporter:** add configurable span dimensions ([#23230](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/23230)) ([d0fe5f3](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/d0fe5f310ca8be484e5c83c9c772144d322e5859))
* **influxdbexporter:** limit size of payload ([#24001](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/24001)) ([4e3c466](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/4e3c466b890ec08b4311661b5da1b170a659b588))
* **internal/metadataproviders/system:** do not return empty host.id ([#24239](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/24239)) ([7e6145c](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/7e6145c00ed6afb56eb954ac4a4efcfbc2d4c1be)), closes [#24230](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/24230)
* **internal/stanza:** add remove operator ([#9524](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/9524)) ([ea328de](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/ea328de0a0a8a1c327c042b2582a73fa31866e2c))
* **journaldreceiver:** fail if unsufficient permissions for journalctl command ([#23276](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/23276)) ([37a8518](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/37a8518550eb63dce0e6c207ae4c1dba1ba4de87)), closes [#20906](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/20906)
* k8sclusterreceiver allow disabling metrics ([#24657](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/24657)) ([cce00b7](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/cce00b74622423ac7b5d41f56ecbe36d01c7959a))
* **mysqlreceiver:** add mysql.tmp_resources metric ([#14726](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/14726)) ([e80df4f](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/e80df4fed97ae9eacffe18339f9238a19179e68c))
* **mysqlreceiver:** deprecate `mysql. locked_connects` in favor of `mysql.connection.errors` ([#23274](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/23274)) ([1c84fb2](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/1c84fb2912908270d8997505455252c316cf2072))
* **mysqlreceiver:** removing `mysql.locked_connects` metric which is replaced by `mysql.connection.errors` ([#23990](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/23990)) ([e5cd1ca](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/e5cd1ca7bcdb8dd099c84191f8dd977d997efaf7)), closes [#23211](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/23211)
* **operator/recombine:** do not combine logs before first_entry matches line ([#416](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/416)) ([fc8a875](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/fc8a8752488b83e4b4478645a75eff1138f64382))
* **processors/k8sattributesprocessor:** support metadata enrichment based on multiple attributes ([#8465](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/8465)) ([4cf86de](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/4cf86dee2359683a4cf0bf71668e00018e76202b))
* **recombine:** add `combine_with` option ([#315](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/315)) ([0106b0d](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/0106b0da814e1a3e228cc727e34652c1191b3174)), closes [#314](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/314)
* remove elasticsearch feature gates for indices and cluster shards ([#18189](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/18189)) ([29d3a8c](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/29d3a8c5620b3d2ce4c42a64a5fa9ae8646994e7))
* **routingprocessor:** add option to drop resource attribute used for routing ([#8990](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/8990)) ([8bbf27c](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/8bbf27c04c3919427f614f57ee23b24670708ff6))
* **routingprocessor:** allow routing for all signals ([#5869](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/5869)) ([95114fb](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/95114fbb82343a475c3232d73ce4a23c447358d3))
* **routingprocessor:** route on resource attributes ([#5694](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/5694)) ([3bf6a49](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/3bf6a49fe6201ec750416d244c09c0935c9459c7)), closes [#5538](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/5538)
* **statsdReceiver:** add name and version to scope ([#23564](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/23564)) ([c40be10](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/c40be10a6e75e5ea15916d1dfeec95bf4de7a32c))
* **syslogreceiver:** validate protocol name ([#27581](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/27581)) ([8cd3f10](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/8cd3f10e9e70f4649475756bb683c9ecf579f9e5))
* trim whitechars for file with multiline ([#212](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/212)) ([2ffe032](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/2ffe032d299e2bbe7f0f46ad33b2b370ee0259df))
* update context to take time ([#26371](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/26371)) ([c276c0e](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/c276c0e3588b491a3c6e8d7b228f3b1c5ec93b37)), closes [#22010](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/22010)


### Bug Fixes

* “pointer to empty string” is not omitted bug of AWS X-Ray Exporter ([#830](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/830)) ([cb45b36](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/cb45b36c8afb1c2cf17312ca1badbf1ed62758e9))
* append result not assigned to the same slice (gocritic) ([#3179](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/3179)) ([e6deb7f](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/e6deb7f6841e1b7e09ceb8dcc446d08681ae63ff))
* **awscloudwatchreceiver:** emit logs from one stream in one resource ([#22976](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/22976)) ([7ba5b5c](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/7ba5b5c9bafeac0b041f3e6e1eeaf8f44a460805))
* changes conventions.AttributeMessageType to message.type ([#5186](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/5186)) ([94449a8](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/94449a80290f0400cff7a285f53de9c318311c0f))
* decouple mongoDB integration tests ([#16306](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/16306)) ([37c5d76](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/37c5d76849a848d2fd9bbb6edb0d5f3a433ab1e1))
* drop a metric if it has a staleness flag ([#6977](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/6977)) ([fddf498](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/fddf49870fd99b8ea0f1453b78a7f21e4984ebeb))
* **exporter/sentry:** fix sentry tracing not working ([#4320](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/4320)) ([6283004](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/628300424c49b36709066cb74cd7bd3bb1f57adf)), closes [#4141](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/4141) [#4141](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/4141)
* fails to start if at least one resourcedetection detector returns an error ([#5242](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/5242)) ([4ee9be0](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/4ee9be0e491262becba076db70d6174d691475cc))
* **helpers/multiline:** fix force flushing with multiline ([#434](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/434)) ([6bc7c94](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/6bc7c94635ce03bf6182d56aec7ef43bba798cf7))
* Incorrect conversion between integer types ([#6939](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/6939)) ([95ae537](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/95ae53705f60971708d4b44a9c1407c8da8bbf9d))
* **influxdbexporter:** handle empty attribute values ([#21526](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/21526)) ([da70db7](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/da70db73c6a3d7fafdae1127f81802a405c81bde))
* **input/file,docs:** fix typo for file attributes names ([#208](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/208)) ([f24ed9d](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/f24ed9d551a00af30ff87d99200e669b89dad9a7))
* **k8sprocessor:** check every association for eventual update ([#14097](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/14097)) ([ef92f08](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/ef92f089c19ec1f1dee59cfa8ae5591acd3001de))
* **k8sprocessor:** fix passthrough mode ([#13993](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/13993)) ([64d03ac](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/64d03acf8c399f993b1ae7bb2802bacc188e6f4e))
* **kafkareceiver:** use logs obsrecv op instead of traces op ([#6431](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/6431)) ([9f04782](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/9f04782839fb8133baf121d5e2858cd1eb6d98ff))
* make signalfxexporter a metadata exporter ([#1252](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/1252)) ([0991362](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/0991362c676491a3b893222081bfa8aaa56b7697))
* **pkg/stanza/input/file/reader:** skip building fingerprint in case of configuration change ([#10485](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/10485)) ([1db76fe](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/1db76fe08f076860dca5ee7cf846b315c6afb63a))
* **recombine:** change default operator type ([#317](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/317)) ([2645572](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/26455728d851aa8e8fd13320abbc3e85491cb74b))
* remove squash on configtls.TLSClientSetting for AWS components ([#5454](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/5454)) ([5118335](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/5118335535117137bd175ac4ed9cd48fe529895f))
* remove squash on configtls.TLSClientSetting for elastic components ([#5539](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/5539)) ([7b60adb](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/7b60adb65f10f6da209b4110160f1e4ea2889284))
* remove squash on configtls.TLSClientSetting for observiqexporter ([#5540](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/5540)) ([79d95af](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/79d95af138ef4060adb1f5950eb628b4efce5aca)), closes [#5433](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/5433)
* remove squash on configtls.TLSClientSetting for splunkhecexporter ([#5541](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/5541)) ([7fecacb](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/7fecacb5f0b841d11e1013b57e51df91d0a98104)), closes [#5433](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/5433)
* **stanza/converter:** handle syslogrecevier attributes correctly ([#27415](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/27415)) ([4eca940](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/4eca940ec6f69b204bd8947d1a0f405ac2aaa77e))
* tracegen example docker-compose and documentation ([#17415](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/17415)) ([6dbc0bb](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/6dbc0bb3b40128cd1b04adac871310c9a1a768a6))
* typo in transactions metric, collect transactions ([#12085](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/12085)) ([3d5850a](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/3d5850a348f1ad656269b15818ca7a4200d06723))
* **vCenter:** balooned memory is expressend in MB ([#16729](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/16729)) ([dd480e2](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/dd480e2d4da2a5ff1930635155167cf8b71613bf))


### jmx

* Add endpoint config field ([#1550](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/1550)) ([4a547be](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/4a547be18ec1503f5bb66b5c8573f418593dcd68))


* [chore] Remove name mapping in hec (#16612) ([bdd162a](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/bdd162a621018c0a593440c98d9df4d91c3b45a2)), closes [#16612](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/16612)
* [exporters/signalfx] breaking change: do not convert periods to underscores in dimension keys (#2456) ([b1cb781](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/b1cb781428fed53aa9637f99eb6b2365590c8151)), closes [#2456](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/2456)
* BREAKING CHANGE: Rename kinesis exporter to awskinesis (#2234) ([9677f5b](https://github.com/bhanuba/opentelemetry-collector-contrib/commit/9677f5b3a3cca335aae23938de07d2296468572e)), closes [#2234](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/2234) [#2164](https://github.com/bhanuba/opentelemetry-collector-contrib/issues/2164)
