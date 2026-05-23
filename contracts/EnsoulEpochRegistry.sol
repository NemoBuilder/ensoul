// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @title EnsoulEpochRegistry — V4 epoch root anchor
/// @notice Records one Merkle root per (galaxyId, index) so the off-chain
///         distillation pipeline can be audited. Only the platform writer
///         can record roots; anyone can read.
///
/// Design notes:
///   - galaxyId = bytes32 (the off-chain Galaxy UUID, encoded big-endian).
///     Using a generic bytes32 means we don't need a separate Galaxy NFT
///     deployed first to start anchoring roots.
///   - index is monotonically increasing per galaxy; the contract enforces
///     strict +1 so a missed batch cannot silently overwrite history.
///   - The global epoch stream uses galaxyId = bytes32(0).
///
/// Phase 2.1 only stores roots. Phase 3 will layer SLA / dispute on top.
contract EnsoulEpochRegistry {
    /// @dev keccak256 of off-chain canonical atom JSON, batched into a Merkle root.
    struct Epoch {
        bytes32 root;
        uint64  atomCount;
        uint64  closedAt;     // block.timestamp at recordRoot()
        address writer;
    }

    /// @notice galaxyId => index => Epoch
    mapping(bytes32 => mapping(uint64 => Epoch)) public epochs;

    /// @notice galaxyId => next expected index (starts at 1)
    mapping(bytes32 => uint64) public nextIndex;

    /// @notice owner can rotate writer / transfer ownership
    address public owner;

    /// @notice the only address allowed to call recordRoot()
    address public writer;

    event RootRecorded(
        bytes32 indexed galaxyId,
        uint64  indexed index,
        bytes32 root,
        uint64  atomCount,
        uint64  closedAt,
        address writer
    );
    event WriterRotated(address indexed oldWriter, address indexed newWriter);
    event OwnerTransferred(address indexed oldOwner, address indexed newOwner);

    modifier onlyOwner() {
        require(msg.sender == owner, "not owner");
        _;
    }

    modifier onlyWriter() {
        require(msg.sender == writer, "not writer");
        _;
    }

    constructor(address _writer) {
        owner = msg.sender;
        writer = _writer;
        emit OwnerTransferred(address(0), msg.sender);
        emit WriterRotated(address(0), _writer);
    }

    /// @notice Anchor one epoch root.
    /// @param galaxyId Off-chain Galaxy UUID encoded as bytes32 (or 0x0 for global).
    /// @param index    Must equal nextIndex[galaxyId] (starts at 1).
    /// @param root     Merkle root over the canonical atom hashes in this epoch.
    /// @param atomCount Number of atoms covered (for stats).
    function recordRoot(
        bytes32 galaxyId,
        uint64 index,
        bytes32 root,
        uint64 atomCount
    ) external onlyWriter {
        uint64 expected = nextIndex[galaxyId];
        if (expected == 0) expected = 1;
        require(index == expected, "bad index");
        require(root != bytes32(0), "zero root");

        epochs[galaxyId][index] = Epoch({
            root: root,
            atomCount: atomCount,
            closedAt: uint64(block.timestamp),
            writer: msg.sender
        });
        nextIndex[galaxyId] = index + 1;

        emit RootRecorded(galaxyId, index, root, atomCount, uint64(block.timestamp), msg.sender);
    }

    /// @notice Off-chain Merkle verification helper. Sibling path is left-to-right
    ///         from leaf level upward. Pairing rule mirrors the off-chain code
    ///         (Bitcoin-style: odd levels duplicate the last node).
    function verifyLeaf(
        bytes32 galaxyId,
        uint64 index,
        bytes32 leaf,
        bytes32[] calldata path,
        uint256 leafIndex
    ) external view returns (bool) {
        bytes32 root = epochs[galaxyId][index].root;
        if (root == bytes32(0)) return false;
        bytes32 cur = leaf;
        uint256 idx = leafIndex;
        for (uint256 i = 0; i < path.length; i++) {
            if (idx % 2 == 0) {
                cur = sha256(abi.encodePacked(cur, path[i]));
            } else {
                cur = sha256(abi.encodePacked(path[i], cur));
            }
            idx /= 2;
        }
        return cur == root;
    }

    /// @notice Rotate the writer key (e.g. after a key compromise).
    function setWriter(address newWriter) external onlyOwner {
        require(newWriter != address(0), "zero writer");
        emit WriterRotated(writer, newWriter);
        writer = newWriter;
    }

    /// @notice Transfer ownership. Two-step transfer is out of scope for V4.0;
    ///         use a multisig as owner if you care about the safety margin.
    function transferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "zero owner");
        emit OwnerTransferred(owner, newOwner);
        owner = newOwner;
    }
}
