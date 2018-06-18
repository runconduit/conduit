use std::{error, fmt};
use std::net::SocketAddr;
use std::time::Duration;
use std::sync::Arc;

use http;
use futures::{Async, Poll};
use tower_service as tower;
use tower_balance::{choose, load, Balance};
use tower_buffer::Buffer;
use tower_discover::{Change, Discover};
use tower_in_flight_limit::InFlightLimit;
use tower_h2;
use tower_h2_balance::{PendingUntilFirstData, PendingUntilFirstDataBody};
use conduit_proxy_router::Recognize;

use bind::{self, Bind, Protocol};
use control::destination::{self, Bind as BindTrait, Resolution};
use ctx;
use telemetry::sensor::http::{ResponseBody as SensorBody};
use timeout::Timeout;
use transparency::{h1, HttpBody};
use transport::{DnsNameAndPort, Host, HostAndPort};

type BindProtocol<B> = bind::BindProtocol<Arc<ctx::Proxy>, B>;

pub struct Outbound<B> {
    bind: Bind<Arc<ctx::Proxy>, B>,
    discovery: destination::Resolver,
    bind_timeout: Duration,
}

const MAX_IN_FLIGHT: usize = 10_000;

/// This default is used by Finagle.
const DEFAULT_DECAY: Duration = Duration::from_secs(10);

#[derive(Clone, Debug, PartialEq, Eq, Hash)]
pub enum Destination {
    Hostname(DnsNameAndPort),
    ImplicitOriginalDst(SocketAddr),
}

// ===== impl Outbound =====

impl<B> Outbound<B> {
    pub fn new(bind: Bind<Arc<ctx::Proxy>, B>,
               discovery: destination::Resolver,
               bind_timeout: Duration)
               -> Outbound<B> {
        Self {
            bind,
            discovery,
            bind_timeout,
        }
    }


    /// TODO: Return error when `HostAndPort::normalize()` fails.
    /// TODO: Use scheme-appropriate default port.
    fn normalize(authority: &http::uri::Authority) -> Option<HostAndPort> {
        const DEFAULT_PORT: Option<u16> = Some(80);
        HostAndPort::normalize(authority, DEFAULT_PORT).ok()
    }

    /// Determines the logical host:port of the request.
    ///
    /// If the parsed URI includes an authority, use that. Otherwise, try to load the
    /// authority from the `Host` header.
    ///
    /// The port is either parsed from the authority or a default of 80 is used.
    fn host_port(req: &http::Request<B>) -> Option<HostAndPort> {
        // Note: Calls to `normalize` cannot be deduped without cloning `authority`.
        req.uri()
            .authority_part()
            .and_then(Self::normalize)
            .or_else(|| {
                h1::authority_from_host(req)
                    .and_then(|h| Self::normalize(&h))
            })
    }

    /// Determines the destination to use in the router.
    ///
    /// A Destination is determined for each request as follows:
    ///
    /// 1. If the request's authority contains a logical hostname, it is routed by the
    ///    hostname and port. If an explicit port is not provided, a default is assumed.
    ///
    /// 2. If the request's authority contains a concrete IP address, it is routed
    ///    directly to that IP address.
    ///
    /// 3. If no authority is present on the request, the client's original destination,
    ///    as determined by the SO_ORIGINAL_DST socket option, is used as the request's
    ///    destination.
    ///
    ///    The SO_ORIGINAL_DST socket option is typically set by iptables(8) in
    ///    containerized environments like Kubernetes (as configured by the proxy-init
    ///    program).
    ///
    /// 4. If the request has no authority and the SO_ORIGINAL_DST socket option has not
    ///    been set (or is unavailable on the current platform), no destination is
    ///    determined.
    fn destination(req: &http::Request<B>) -> Option<Destination> {
        match Self::host_port(req) {
            Some(HostAndPort { host: Host::DnsName(host), port }) => {
                let dst = DnsNameAndPort { host, port };
                Some(Destination::Hostname(dst))
            }

            Some(HostAndPort { host: Host::Ip(ip), port }) => {
                let dst = SocketAddr::from((ip, port));
                Some(Destination::ImplicitOriginalDst(dst))
            }

            None => {
                req.extensions()
                    .get::<Arc<ctx::transport::Server>>()
                    .and_then(|ctx| ctx.orig_dst_if_not_local())
                    .map(Destination::ImplicitOriginalDst)
            }
        }
    }
}

impl<B> Clone for Outbound<B>
where
    B: tower_h2::Body + Send + 'static,
    B::Data: Send,
{
    fn clone(&self) -> Self {
        Self {
            bind: self.bind.clone(),
            discovery: self.discovery.clone(),
            bind_timeout: self.bind_timeout.clone(),
        }
    }
}

impl<B> Recognize for Outbound<B>
where
    B: tower_h2::Body + Send + 'static,
    <B::Data as ::bytes::IntoBuf>::Buf: Send,
{
    type Request = http::Request<B>;
    type Response = http::Response<PendingUntilFirstDataBody<
        load::peak_ewma::Handle,
        SensorBody<HttpBody>,
    >>;
    type Error = <Self::Service as tower::Service>::Error;
    type Key = (Destination, Protocol);
    type RouteError = bind::BufferSpawnError;
    type Service = InFlightLimit<Timeout<Buffer<Balance<
        load::WithPeakEwma<Discovery<B>, PendingUntilFirstData>,
        choose::PowerOfTwoChoices,
    >>>>;

    // Route the request by its destination AND PROTOCOL. This prevents HTTP/1
    // requests from being routed to HTTP/2 servers, and vice versa.
    fn recognize(&self, req: &Self::Request) -> Option<Self::Key> {
        let dest = Self::destination(req)?;
        let proto = bind::Protocol::detect(req);
        Some((dest, proto))
    }

    /// Builds a dynamic, load balancing service.
    ///
    /// Resolves the authority in service discovery and initializes a service that buffers
    /// and load balances requests across.
    fn bind_service(
        &self,
        key: &Self::Key,
    ) -> Result<Self::Service, Self::RouteError> {
        let &(ref dest, ref protocol) = key;
        debug!("building outbound {:?} client to {:?}", protocol, dest);

        let resolve = match *dest {
            Destination::Hostname(ref authority) => {
                Discovery::NamedSvc(self.discovery.resolve(
                    authority,
                    self.bind.clone().with_protocol(protocol.clone()),
                ))
            },
            Destination::ImplicitOriginalDst(addr) => {
                Discovery::ImplicitOriginalDst(Some((addr, self.bind.clone()
                    .with_protocol(protocol.clone()))))
            }
        };

        let balance = {
            let instrument = PendingUntilFirstData::default();
            let loaded = load::WithPeakEwma::new(resolve, DEFAULT_DECAY, instrument);
            Balance::p2c(loaded)
        };

        let log = ::logging::proxy().client("out", Dst(dest.clone()))
            .with_protocol(protocol.clone());
        let buffer = Buffer::new(balance, &log.executor())
            .map_err(|_| bind::BufferSpawnError::Outbound)?;

        let timeout = Timeout::new(buffer, self.bind_timeout);

        Ok(InFlightLimit::new(timeout, MAX_IN_FLIGHT))
    }
}

pub enum Discovery<B> {
    NamedSvc(Resolution<BindProtocol<B>>),
    ImplicitOriginalDst(Option<(SocketAddr, BindProtocol<B>)>),
}

impl<B> Discover for Discovery<B>
where
    B: tower_h2::Body + Send + 'static,
    <B::Data as ::bytes::IntoBuf>::Buf: Send,
{
    type Key = SocketAddr;
    type Request = http::Request<B>;
    type Response = bind::HttpResponse;
    type Error = <Self::Service as tower::Service>::Error;
    type Service = bind::Service<B>;
    type DiscoverError = BindError;

    fn poll(&mut self) -> Poll<Change<Self::Key, Self::Service>, Self::DiscoverError> {
        match *self {
            Discovery::NamedSvc(ref mut w) => w.poll()
                .map_err(|_| BindError::Internal),
            Discovery::ImplicitOriginalDst(ref mut opt) => {
                // This "discovers" a single address for an external service
                // that never has another change. This can mean it floats
                // in the Balancer forever. However, when we finally add
                // circuit-breaking, this should be able to take care of itself,
                // closing down when the connection is no longer usable.
                if let Some((addr, bind)) = opt.take() {
                    let svc = bind.bind(&addr.into())
                        .map_err(|_| BindError::External { addr })?;
                    Ok(Async::Ready(Change::Insert(addr, svc)))
                } else {
                    Ok(Async::NotReady)
                }
            }
        }
    }
}
#[derive(Copy, Clone, Debug)]
pub enum BindError {
    External { addr: SocketAddr },
    Internal,
}

impl fmt::Display for BindError {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match *self {
            BindError::External { addr } =>
                write!(f, "binding external service for {:?} failed", addr),
            BindError::Internal =>
                write!(f, "binding internal service failed"),
        }
    }

}

impl error::Error for BindError {
    fn description(&self) -> &str {
        match *self {
            BindError::External { .. } => "binding external service failed",
            BindError::Internal => "binding internal service failed",
        }
    }

    fn cause(&self) -> Option<&error::Error> { None }
}

struct Dst(Destination);

impl fmt::Display for Dst {
    fn fmt(&self, f: &mut fmt::Formatter) -> fmt::Result {
        match self.0 {
            Destination::Hostname(ref name) => {
                write!(f, "{}:{}", name.host, name.port)
            }
            Destination::ImplicitOriginalDst(ref addr) => addr.fmt(f),
        }
    }
}
