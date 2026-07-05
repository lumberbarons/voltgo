# Frame corpus

Response frames captured from real batteries, one directory per model. Every
parser change is validated against every frame here, so captures from other
models and firmware revisions directly extend the library's compatibility
guarantees — no hardware in CI required.

## Contributing a capture

1. Create a directory named after the battery model (lowercase, e.g.
   `zt-25.6v100ah`).
2. Add one `.hex` file per captured response frame. Files whose name starts
   with `status` must be responses to the standard status poll (read 41
   registers from address 0) and get plausibility-checked field by field.
3. Format: hex-encoded bytes of the complete frame including address,
   function, payload, and CRC. Whitespace and newlines are ignored; lines
   starting with `#` are comments — use them to record the battery model,
   state (idle/charging/discharging), and how the frame was captured.

Captures can come from `voltgo-cli read`, Wireshark/btmon, or an Android HCI
snoop log (use a full btsnoop capture, not the truncated bugreport log).
