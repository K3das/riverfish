# Riverfish

this was a proof of concept written in late 2022 for classifying domains coming out of CT logs

- `./ct-collection` - CT scanning service
- `./ct-streamer` - module for scanning CT logs, used by collection
- `./scoring` - scoring service, consumes kafka certificate stream, publishes to an output stream
- `./output` - screenshots and notifications of detections

used this to figure out protobufs and kafka, and in 2022 there were less far less feasible solutions for doing this. **i dont recommend using any of this, use the better solutions out there**, i am publishing this just as a POC, plz dont really use this