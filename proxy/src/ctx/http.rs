use http;
use std::sync::Arc;

use control::discovery::DstLabelsWatch;
use ctx;
use telemetry::metrics::DstLabels;


/// Describes a stream's request headers.
#[derive(Clone, Debug, PartialEq, Eq, Hash)]
pub struct Request {
    // A numeric ID useful for debugging & correlation.
    pub id: usize,

    pub uri: http::Uri,
    pub method: http::Method,

    /// Identifies the proxy server that received the request.
    pub server: Arc<ctx::transport::Server>,

    /// Identifies the proxy client that dispatched the request.
    pub client: Arc<ctx::transport::Client>,

    /// Optional information on the request's destination service, which may
    /// be provided by the control plane for destinations lookups against its
    /// discovery API.
    pub dst_labels: Option<DstLabels>,
}

/// Describes a stream's response headers.
#[derive(Clone, Debug, PartialEq, Eq, Hash)]
pub struct Response {
    pub request: Arc<Request>,

    pub status: http::StatusCode,
}

// TODO Describe a request's EOS.
//pub struct EndRequest {
//    pub response: Arc<Request>,
//
//    pub h2_error_code: Option<u32>,
//}

impl Request {
    pub fn new<B>(
        request: &http::Request<B>,
        server: &Arc<ctx::transport::Server>,
        client: &Arc<ctx::transport::Client>,
        id: usize,
    ) -> Arc<Self> {
        // Look up whether the request has been extended with optional
        // destination labels from the control plane's discovery API.
        let dst_labels = request
            .extensions()
            .get::<DstLabels>()
            .cloned();
        let r = Self {
            id,
            uri: request.uri().clone(),
            method: request.method().clone(),
            server: Arc::clone(server),
            client: Arc::clone(client),
            dst_labels,
        };

        Arc::new(r)
    }

    pub fn dst_labels(&self) -> Option<&DstLabelsWatch> {
        self.client.dst_labels.as_ref()
    }
}

impl Response {
    pub fn new<B>(response: &http::Response<B>, request: &Arc<Request>) -> Arc<Self> {
        let r = Self {
            status: response.status(),
            request: Arc::clone(request),
        };

        Arc::new(r)
    }
}
