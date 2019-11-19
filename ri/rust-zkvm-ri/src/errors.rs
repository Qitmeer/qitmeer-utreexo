/// Represents an error in proof creation, verification, or parsing.
#[derive(Fail, Clone, Debug, Eq, PartialEq)]
pub enum VMError {
    /// This error occurs when a Merkle proof of inclusion is invalid.
    #[fail(display = "Invalid Merkle proof.")]
    InvalidMerkleProof,
}
