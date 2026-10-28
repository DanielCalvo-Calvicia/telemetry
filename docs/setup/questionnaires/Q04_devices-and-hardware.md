# Q04 — Devices and hardware

**Status: answered by the maintainer on 2026-10-27; the points left open are in [Q07](Q07_open-points-after-the-answers.md).** Proposal: [PR-001](../../proposals/PR-001-telemetry-standard-go-python.md) (draft v0.1, nothing built). This questionnaire settles D-03 and the scope of section 8 of the specification. D-05 (hardware libraries) was closed on 2026-10-27: the library has no hardware drivers (see questions 2 and 3).

Write your answer after `A:`; `UNKNOWN`, `NOT APPLICABLE` and `LATER` are valid; never write secrets, host names or account ids (the repository is public). Each question explains the point and has a **default**: an empty answer means the default is accepted.

---

1. **Which hardware projects exist today, and in which language is each one written?** The source helpers of REQ-DEV-011 (GPIO, I2C, SPI, serial, camera, motor, vendor SDK) are a guess until they are checked against real hardware.
   A:Right now there are manu actuators and sensors on a Raspberry pi made on python.

2. **D-05: which driver library is used for each adapter, per language?** *Closed by the maintainer on 2026-10-27.* The library has no hardware drivers: the project that owns the hardware keeps its own drivers and objects. What differs per source (GPIO, I2C, ...) is the input of a typed helper; the output is the same generic event, so one UI shows all of them (PR-001 8.4).
   A:Closed (no library to choose).

3. **Does the library talk to the hardware, or only report about it?** *Answered by the maintainer on 2026-10-27:* option (b). The `outbound/device_control/` adapters were removed from PR-001; the consumer drives the hardware with its own code and reports through the library.
   A:(b) only observe.

4. **Sampling rates.** REQ-DEV-009 asks for batching and configurable sampling for high-frequency readings. What is the highest rate of a real sensor (readings per second), and how many sensors does one device have?
   A:I dont knwo but some microcontroles will need to run at least 50ms

5. **Offline devices.** `offline_device` implies a spool that lasts hours or days. How long can a device stay offline, and how much disk does it have?
   A:It must stay as long as needed if the operator doesnt clean it up or the project that is using it doesnt call the function to clean it up its their problem. There is no specified disk usage, but i think 5GB is enough, this will be plenty.
   > **Update 2026-10-28:** A quota stays (Q07-11): 256 MiB per destination by default, and a public cleanup function; the project cleans the rest.

6. **D-03: SQL dialects.**
   Options: (a) no SQL adapter in v1; events go to HTTP or a file (my default); (b) PostgreSQL; (c) SQL Server; (d) write which.
   *Default: (a).*
   A:Lets start with sqlite, if there needs an extra one it will be added later
   > **Update 2026-10-28:** Refined by Q07-12, Q08-5 and Q09-5: SQLite is a destination whose layout is part of `contracts/`; a reader only reads it.

7. **Is the device module in v1?**
   Options: (a) yes, with the generic contracts first and only the source helpers of the pilot hardware (my default); (b) all the source helpers in v1; (c) after the core is released.
   *Default: (a).*
   A:Generate some events yourself. Then it will be edit depending on the real projects
