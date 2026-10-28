# config

The schema of the configuration (T-03.6, REQ-CFG-001): the option names, the types, the defaults and the profiles (`microservice`, `cronjob`, `worker`, `robotics`, `offline_device`). One logical model for both languages. The library reads no file and no environment (ADR-004): the consumer loads its configuration and passes it in, so this folder describes the shape and never a value of a real project. Empty for now.
