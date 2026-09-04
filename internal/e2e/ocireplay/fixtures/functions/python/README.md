# Functions replay image

This minimal Python FDK image is the reusable invocation image for recording
the Functions `Function` lifecycle. It returns the supplied payload and does
not access OCI services.

Publish it to a public OCIR repository in the recording region:

```bash
docker buildx build \
  --platform linux/amd64 \
  --tag "$OCI_REPLAY_FUNCTION_IMAGE" \
  --push .
```

The recording test stores the image and application-specific invocation
endpoints as cassette bindings, so checked-in fixtures contain no operator
registry namespace.
