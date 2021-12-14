# proxyU Integration Protocol Specification

This is the protocol specification for integrating a data processor (e.g. a
company, governmental organization, non-profit, etc) to the dataU network,
through proxyU. For further documentation about dataU and its architecture
please refer to the [official
documentation](https://github.com/jibecompany/docsu/).

## Integration Protocol

This protocol uses [gRPC](https://grpc.io/). This protocol supports the right
balance between performance, asynchronicity, ease of use, and programming
language support. The API exposed over this protocol can be accessed through
either TCP or a Unix domain (`SOCK_STREAM`) socket, made available by the
proxyU application. It only uses named parameters and only uses RPC methods, no
notifications.

## Security

From dataU's standpoint, the security stops at proxyU. Once the communication
is secure between proxyU instances and the data is delivered to the right
instance, dataU's job is done. The end-to-end encryption of data is between
proxyU instances, not between arbitrary applications of data processors.

It is up to the user of proxyU to properly secure the integration API endpoint.
Recommended is to run proxyU on the same machine as your application, and using
a Unix domain socket with minimized access rights (which is the default). This
approach is both faster and more secure. If this is not possible, proxyU offers
the ability to load a TLS certificate in order to secure the TCP socket with
mutually-authenticated TLS. Even though the protocol is secured with TLS, it is
not recommended to expose this to the internet.

## Integration API

This is the gRPC API exposed over the Integration Protocol. Authorization is
done by proxyU and you, as an integrator, can assume safely that every incoming
RPC request is authenticated and authorized.

For example, when proxyU asks your application for the phone number of a data
subject, that data subject has given their explicit consent to have that
retrieved from your application.

The protocol specification is defined in `protocol/proxyu.proto`.

### Identification of the Data Subject

Data subjects are identified using their public key. This public key is an
ed25519 public key and is 32 bytes long. In order to communicate with dataU, a
data processor needs to identify the data subject and store their public key as
an identifier on their local system.

Correlation is implemented as a bi-directional stream. You can send correlation
requests and you will receive correlation messages and public keys in return.

First, initiate a stream using the `Correlation` RPC call, in which you send a
`CorrelationRequest`. The response is a stream in which you will receive
exactly two messages. After those, you can close the stream. It is
automatically closed after one hour, giving the data subject ample time to
correlate with you.

| Message |
| --- |
| `CorrelationRequest` |

You will get the following message back:

| Message | `correlation_message` |
| --- | --- |
| `CorrelationResponse` | String; Correlation message you must encode into a QR-code for the data subject to scan |

Once the data subject has scanned this QR-code, dashboardU will contact your
proxyU, causing it to contact your application with the following
`CorrelationResponse`:

| Message |`public_key` |
| --- | --- |
| `CorrelationResponse` | Bytes; 32-byte public key of the data subject |

You do not receive any other information about the data subject, as all other
information is considered protected information for which you need explicit
consent.

You can use this handshake more than once. It is designed to also be used as a
secure single-sign-on mechanism, allowing you to log in and/or register your
users based on the dataU public key.

### Document Management

Documents, often legal policies, need to be submitted to dataU before you can
use them as your terms, conditions, and privacy policy. You are required to
host this document online, over HTTPS. You are allowed to require a TLS client
certificate. The dashboardU will then submit the data subject's certificate
requesting the document. In this way, documents do not necessarily have to be
public to the world.

To submit a document, submit the following RPC call to proxyU using the
`SubmitDocument` function:

| Message | `url` | `hash` |
| --- | --- | --- |
| `SubmitDocumentRequest` | String; the URL (starting with `https://`) pointing to the document you want to submit | Bytes; 32-byte SHA3-256 hash of the document |

The proxyU instance will validate your document. You will get the following
response:

| Message | `ok` | `error` |
| --- | --- | --- |
| `SubmitDocumentResponse` | Boolean; indicates success | String; optional, if `error` is `false` this indicates what is wrong with the document |

### Permission Management

You can request permission to access data from a data subject by sending the
data subject a message in the shape of a QR code, which they can then scan
using the dashboardU application.

Like with data subject correlation, permission management is implemented using
a stream. Open it by calling the `Permission` RPC call and keep it open. You
will get two responses, after which you can close the call. It is automatically
closed after one hour, giving the data subject ample time to respond to the
permission request.

The permission request message can be generated for you by proxyU by sending
this `PermissionRequest`:

| Message | `public_key` | `data` | `process` | `reason` | `policy` | `from` | `until` | `amount` | `level` |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| `PermissionRequest` | Bytes; 32-byte public key of the data subject | Bytes; 16-byte UUIDv4 identifying the data requested in the [data identification graph](https://github.com/jibecompany/docsu#data-identification-graph) | Bytes; 16-byte UUIDv4 value indicating your internal process | Bytes; 16-byte UUIDv4 value of the reason | Bytes; 32-byte SHA3-256 hash of the previously published legal policy | Integer; Unix UTC timestap from when this permission is valid | Integer; Unix UTC timestap until when this permission is valid | Integer; how often this data will be retrieved, `0` sets no limit | Integer; the lowest level of data subject identification required |

You will get the following response:

| Message | `permission_message` |
| --- | --- |
| `PermissionResponse` | String; permission request message you must encode into a QR-code for the data subject to scan  |

Once the data subject grants the permission, you will receive the following
`PermissionResponse`. Notice that you do not see where the requested data will
come from. This is not required for operation of your application and the
correct routing is handled by proxyU.

| Message | `granted` |
| --- | --- |
| `PermissionResponse` | Boolean; whether the permission has been granted |

### Data Management

All data exchange goes through the `Data` RPC call which, as with correlation
and permission handling, is implemented as a bi-directional stream. You can
open this stream by sending a `DataRequest` with a `DataNopRequest`.

The `process` field is entirely optional, and is fine to be kept at all-zero.
It is there to allow the distinction between, for example, separate services
offered by the same data processor. For instance, an insurance company with a
number of insurance policies. This allows a data subject to share different
information per product.

To request data from a data subject, send a `DataRequest` with a
`DataRetrieveRequest`:

| Message | `public_key` | `data` | `process` |
| --- | --- | --- | --- |
| `DataRetrieveRequest` | Bytes; 32-byte public key of the data subject | Bytes; 16-byte UUIDv4 identifying the data requested in the data identification graph | Bytes; 16-byte UUIDv4 value indicating your internal process |

You will receive the following `DataResponse`:

| Message | `public_key` | `data` | `process` | `error` | `fields` |
| --- | --- | --- | --- | --- | --- |
| `DataRetrieveResponse` | Bytes; 32-byte public key of the data subject | Bytes; 16-byte UUIDv4 identifying the data requested in the data identification graph | Bytes; 16-byte UUIDv4 value indicating your internal process | Integer; indicating if it was a success or not | repeated `DataField`; All the data from the leaf nodes of the requested field |

The `DataField` has the following format:

| Message | `uuid` | `value` | `mime` |
| --- | --- | --- | --- |
| `DataField` | Bytes; 16-byte UUIDv4 identifying the data requested in the data identification graph | Bytes; the actual value, format dependent on the MIME-type | String; MIME-type of the `value` |

The same thing works the other way around, if another data processor sends a
request for your data. If proxyU has determined that this request is allowed,
you will receive a `DataRetrieveRequest` in a `DataResponse` for which you need
to send back a `DataRetrieveResponse` in a `DataRequest`. You can leave the
`mime` field empty if you wish, as it will be automatically filled by proxyU on
its way to the other data processor.

A data subject is allowed to supply data directly. Either as a response to a
request from you (there is no other data processor who holds this information),
or as a right to keep their own data updated on your systems. In these
scenarios, you will receive the following `DataResponse`:

| Message | `public_key` | `data` | `process` | `value` | `mime` |
| --- | --- | --- | --- | --- | --- |
| `DataSupplyRequest` | Bytes; 32-byte public key of the data subject | Bytes; 16-byte UUIDv4 identifying the data supplied in the data identification graph | Bytes; 16-byte UUIDv4 value indicating your internal process | Bytes; the new value supplied by the data subject | String; MIME-type of the `value` |

The `data` field will always point to a leaf in the data identification graph.
You must always reply with the following `DataRequest`:

| Message | `public_key` | `data` | `process` | `error` |
| --- | --- | --- | --- | --- |
| `DataSupplyResponse` | Bytes; 32-byte public key of the data subject | Bytes; 16-byte UUIDv4 identifying the data supplied in the data identification graph | Bytes; 16-byte UUIDv4 value indicating your internal process | Integer; indicating whether it was a success or not |

A data subject can request data to be removed from your system. You will
receive this `DataResponse` if a data subject chooses to do so. You will also
receive this `DataResponse` if you are the recipient of that data, fetched from
another data processor. You might not necessarily have a local copy of that
data, but this approach at least gives your the ability to remove the local
copy if you have one.

| Message | `public_key` | `data` | `process` |
| --- | --- | --- | --- |
| `DataDeleteRequest` | Bytes; 32-byte public key of the data subject | Bytes; 16-byte UUIDv4 identifying the data in the data identification graph | Bytes; 16-byte UUIDv4 value indicating your internal process |

The `data` field will always point to a leaf in the data identification graph.
You must always reply with the following `DataRequest`:

| Message | `public_key` | `data` | `process` | `error` |
| --- | --- | --- | --- | --- |
| `DataDeleteResponse` | Bytes; 32-byte public key of the data subject | Bytes; 16-byte UUIDv4 identifying the data in the data identification graph | Bytes; 16-byte UUIDv4 value indicating your internal process | Integer; indicating whether it was a success or not |

#### Error Numbers

Every time the field `error` is used, it should be used with the following values:

| Error number | Meaning |
| --- | --- |
| 0 | OK |
| -1 | Not found |
| -2 | Not allowed for legal reasons |
| -3 | Internal server error |
| -4 | Permission is not found |

Note: these are negative integers!
