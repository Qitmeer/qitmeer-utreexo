//! Encoding utils
//! All methods err using VMError::FormatError for convenience.

use byteorder::{ByteOrder, LittleEndian};

#[derive(Debug)]
pub struct SliceReader<'a> {
    whole: &'a [u8],
    start: usize,
    end: usize,
}

impl<'a> SliceReader<'a> {

}

// Writing API
// This currently writes into the Vec, but later can be changed to support Arenas to minimize allocations

// Writes a single byte
pub(crate) fn write_u8<'a>(x: u8, target: &mut Vec<u8>) {
    target.push(x);
}

// Writes a LE32-encoded integer
pub(crate) fn write_u32<'a>(x: u32, target: &mut Vec<u8>) {
    let mut buf = [0u8; 4];
    LittleEndian::write_u32(&mut buf, x);
    target.extend_from_slice(&buf);
}

// Writes a LE64-encoded integer
pub(crate) fn write_u64<'a>(x: u64, target: &mut Vec<u8>) {
    let mut buf = [0u8; 8];
    LittleEndian::write_u64(&mut buf, x);
    target.extend_from_slice(&buf);
}

// Writes a usize as a LE32-encoded integer
pub(crate) fn write_size<'a>(x: usize, target: &mut Vec<u8>) {
    write_u32(x as u32, target);
}

/// Writes a 32-byte array and returns the subsequent slice
pub(crate) fn write_bytes(x: &[u8], target: &mut Vec<u8>) {
    target.extend_from_slice(&x);
}


/// A trait for consensus-critical encoding format for encoding data structures.
/// Note: serde is not used for consesus-critical operations.
pub trait Encodable {
    /// Encodes receiver into bytes appending them to a provided buffer.
    fn encode(&self, buf: &mut Vec<u8>);
    /// Returns precise length in bytes for the serialized representation of the receiver.
    fn serialized_length(&self) -> usize;
    /// Encodes the receiver into a newly allocated vector of bytes.
    fn encode_to_vec(&self) -> Vec<u8> {
        let mut buf = Vec::with_capacity(self.serialized_length());
        self.encode(&mut buf);
        buf
    }
}
