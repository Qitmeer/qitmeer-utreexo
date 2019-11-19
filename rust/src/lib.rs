#![deny(missing_docs)]
//! Utreexo implementation.

#[macro_use]
extern crate failure;
extern crate serde;

#[macro_use]
mod serialization;
mod merkle;
mod encoding;
mod errors;
pub mod utreexo;
