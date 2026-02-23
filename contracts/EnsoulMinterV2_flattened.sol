// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

// ═══════════════════════════════════════════════════════════════════════
// Flattened for BscScan verification
// Source: OpenZeppelin Contracts v5.x  +  EnsoulMinterV2
// ═══════════════════════════════════════════════════════════════════════

// ── OpenZeppelin: Context ──────────────────────────────────────────────

abstract contract Context {
    function _msgSender() internal view virtual returns (address) {
        return msg.sender;
    }

    function _msgData() internal view virtual returns (bytes calldata) {
        return msg.data;
    }

    function _contextSuffixLength() internal view virtual returns (uint256) {
        return 0;
    }
}

// ── OpenZeppelin: Ownable ──────────────────────────────────────────────

abstract contract Ownable is Context {
    address private _owner;

    error OwnableUnauthorizedAccount(address account);
    error OwnableInvalidOwner(address owner);

    event OwnershipTransferred(address indexed previousOwner, address indexed newOwner);

    constructor(address initialOwner) {
        if (initialOwner == address(0)) {
            revert OwnableInvalidOwner(address(0));
        }
        _transferOwnership(initialOwner);
    }

    modifier onlyOwner() {
        _checkOwner();
        _;
    }

    function owner() public view virtual returns (address) {
        return _owner;
    }

    function _checkOwner() internal view virtual {
        if (owner() != _msgSender()) {
            revert OwnableUnauthorizedAccount(_msgSender());
        }
    }

    function renounceOwnership() public virtual onlyOwner {
        _transferOwnership(address(0));
    }

    function transferOwnership(address newOwner) public virtual onlyOwner {
        if (newOwner == address(0)) {
            revert OwnableInvalidOwner(address(0));
        }
        _transferOwnership(newOwner);
    }

    function _transferOwnership(address newOwner) internal virtual {
        address oldOwner = _owner;
        _owner = newOwner;
        emit OwnershipTransferred(oldOwner, newOwner);
    }
}

// ── OpenZeppelin: IERC721Receiver ──────────────────────────────────────

interface IERC721Receiver {
    function onERC721Received(
        address operator,
        address from,
        uint256 tokenId,
        bytes calldata data
    ) external returns (bytes4);
}

// ── OpenZeppelin: ReentrancyGuard ──────────────────────────────────────

abstract contract ReentrancyGuard {
    uint256 private constant NOT_ENTERED = 1;
    uint256 private constant ENTERED = 2;

    uint256 private _status;

    error ReentrancyGuardReentrantCall();

    constructor() {
        _status = NOT_ENTERED;
    }

    modifier nonReentrant() {
        _nonReentrantBefore();
        _;
        _nonReentrantAfter();
    }

    function _nonReentrantBefore() private {
        if (_status == ENTERED) {
            revert ReentrancyGuardReentrantCall();
        }
        _status = ENTERED;
    }

    function _nonReentrantAfter() private {
        _status = NOT_ENTERED;
    }

    function _reentrancyGuardEntered() internal view returns (bool) {
        return _status == ENTERED;
    }
}

// ── OpenZeppelin: ECDSA ────────────────────────────────────────────────

library ECDSA {
    enum RecoverError {
        NoError,
        InvalidSignature,
        InvalidSignatureLength,
        InvalidSignatureS
    }

    error ECDSAInvalidSignature();
    error ECDSAInvalidSignatureLength(uint256 length);
    error ECDSAInvalidSignatureS(bytes32 s);

    function tryRecover(
        bytes32 hash,
        bytes memory signature
    ) internal pure returns (address recovered, RecoverError err, bytes32 errArg) {
        if (signature.length == 65) {
            bytes32 r;
            bytes32 s;
            uint8 v;
            assembly ("memory-safe") {
                r := mload(add(signature, 0x20))
                s := mload(add(signature, 0x40))
                v := byte(0, mload(add(signature, 0x60)))
            }
            return tryRecover(hash, v, r, s);
        } else {
            return (address(0), RecoverError.InvalidSignatureLength, bytes32(signature.length));
        }
    }

    function recover(bytes32 hash, bytes memory signature) internal pure returns (address) {
        (address recovered, RecoverError error_, bytes32 errorArg) = tryRecover(hash, signature);
        _throwError(error_, errorArg);
        return recovered;
    }

    function tryRecover(
        bytes32 hash,
        uint8 v,
        bytes32 r,
        bytes32 s
    ) internal pure returns (address recovered, RecoverError err, bytes32 errArg) {
        if (uint256(s) > 0x7FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF5D576E7357A4501DDFE92F46681B20A0) {
            return (address(0), RecoverError.InvalidSignatureS, s);
        }

        address signer = ecrecover(hash, v, r, s);
        if (signer == address(0)) {
            return (address(0), RecoverError.InvalidSignature, bytes32(0));
        }

        return (signer, RecoverError.NoError, bytes32(0));
    }

    function recover(
        bytes32 hash,
        uint8 v,
        bytes32 r,
        bytes32 s
    ) internal pure returns (address) {
        (address recovered, RecoverError error_, bytes32 errorArg) = tryRecover(hash, v, r, s);
        _throwError(error_, errorArg);
        return recovered;
    }

    function _throwError(RecoverError error_, bytes32 errorArg) private pure {
        if (error_ == RecoverError.NoError) {
            return;
        } else if (error_ == RecoverError.InvalidSignature) {
            revert ECDSAInvalidSignature();
        } else if (error_ == RecoverError.InvalidSignatureLength) {
            revert ECDSAInvalidSignatureLength(uint256(errorArg));
        } else if (error_ == RecoverError.InvalidSignatureS) {
            revert ECDSAInvalidSignatureS(errorArg);
        }
    }
}

// ── OpenZeppelin: MessageHashUtils ─────────────────────────────────────

library MessageHashUtils {
    /**
     * @dev Returns the keccak256 digest of an EIP-191 signed data with version 0x45 (`personal_sign`).
     */
    function toEthSignedMessageHash(bytes32 messageHash) internal pure returns (bytes32 digest) {
        assembly ("memory-safe") {
            mstore(0x00, "\x19Ethereum Signed Message:\n32")
            mstore(0x1c, messageHash)
            digest := keccak256(0x00, 0x3c)
        }
    }
}

// ═══════════════════════════════════════════════════════════════════════
// EnsoulMinterV2
// ═══════════════════════════════════════════════════════════════════════

/// @dev Minimal interface for the ERC-8004 IdentityRegistry.
interface IIdentityRegistry {
    function register(string memory agentURI) external returns (uint256 agentId);
    function safeTransferFrom(address from, address to, uint256 tokenId) external;
}

/// @dev Minimal interface for ERC-20 token rescue.
interface IERC20 {
    function balanceOf(address account) external view returns (uint256);
    function transfer(address to, uint256 amount) external returns (bool);
}

/// @dev Minimal interface for ERC-721 NFT rescue.
interface IERC721 {
    function safeTransferFrom(address from, address to, uint256 tokenId) external;
}

/**
 * @title EnsoulMinterV2
 * @notice Upgraded minter with tiered pricing via backend signature verification.
 *
 * Security model:
 *   - Backend fetches real follower count from Twitter API
 *   - Backend calculates price and signs a permit: hash(handle, price, user, deadline, nonce)
 *   - User calls mint() with the permit; contract verifies signature from trusted signer
 *   - Prevents fake follower count, replay attacks, and front-running
 *
 * Flow:
 *   1. User requests mint permit from backend (POST /api/shell/mint-permit)
 *   2. Backend returns {price, deadline, nonce, signature}
 *   3. User calls mint(agentURI, handleHash, price, deadline, nonce, signature) with msg.value >= price
 *   4. Contract verifies signature, checks nonce, registers NFT, transfers to user
 *   5. BNB fee forwarded to treasury
 */
contract EnsoulMinterV2 is Ownable, IERC721Receiver, ReentrancyGuard {
    using ECDSA for bytes32;
    using MessageHashUtils for bytes32;

    // ── State ──────────────────────────────────────────────────────────
    IIdentityRegistry public immutable registry;
    address public treasury;
    address public trustedSigner;  // Backend platform wallet that signs mint permits
    bool public paused;

    // Nonce tracking to prevent replay attacks
    mapping(address => mapping(uint256 => bool)) public usedNonces;

    // Handle dedup: keccak256(lowercase handle) => true if already minted
    mapping(bytes32 => bool) public mintedHandles;

    // ── Events ─────────────────────────────────────────────────────────
    event Minted(address indexed user, uint256 indexed agentId, bytes32 indexed handleHash, uint256 fee);
    event TrustedSignerUpdated(address oldSigner, address newSigner);
    event TreasuryUpdated(address oldTreasury, address newTreasury);
    event Paused(bool isPaused);

    // ── Errors ─────────────────────────────────────────────────────────
    error InsufficientFee(uint256 required, uint256 provided);
    error MintingPaused();
    error TransferFailed();
    error ZeroAddress();
    error InvalidSignature();
    error ExpiredPermit();
    error NonceAlreadyUsed();
    error HandleAlreadyMinted();

    // ── Constructor ────────────────────────────────────────────────────
    constructor(
        address registry_,
        address treasury_,
        address trustedSigner_
    ) Ownable(msg.sender) {
        if (registry_ == address(0) || treasury_ == address(0) || trustedSigner_ == address(0))
            revert ZeroAddress();
        registry = IIdentityRegistry(registry_);
        treasury = treasury_;
        trustedSigner = trustedSigner_;
    }

    // ── Core ───────────────────────────────────────────────────────────

    /**
     * @notice Mint an ERC-8004 identity NFT with backend-signed pricing.
     * @param agentURI    The agent registration file URI.
     * @param handleHash  keccak256(abi.encodePacked(lowercaseHandle)) for dedup.
     * @param price       The mint price in wei, determined by backend based on follower count.
     * @param deadline    Unix timestamp after which the permit expires.
     * @param nonce       Unique nonce to prevent replay.
     * @param signature   Backend signature of the permit.
     * @return agentId    The newly minted agent's token ID.
     */
    function mint(
        string calldata agentURI,
        bytes32 handleHash,
        uint256 price,
        uint256 deadline,
        uint256 nonce,
        bytes calldata signature
    ) external payable nonReentrant returns (uint256 agentId) {
        if (paused) revert MintingPaused();
        if (block.timestamp > deadline) revert ExpiredPermit();
        if (usedNonces[msg.sender][nonce]) revert NonceAlreadyUsed();
        if (mintedHandles[handleHash]) revert HandleAlreadyMinted();
        if (msg.value < price) revert InsufficientFee(price, msg.value);

        // Verify backend signature
        bytes32 messageHash = keccak256(abi.encodePacked(
            handleHash, price, msg.sender, deadline, nonce, block.chainid, address(this)
        ));
        bytes32 ethSignedHash = messageHash.toEthSignedMessageHash();
        address recovered = ethSignedHash.recover(signature);
        if (recovered != trustedSigner) revert InvalidSignature();

        // Mark nonce and handle as used
        usedNonces[msg.sender][nonce] = true;
        mintedHandles[handleHash] = true;

        // 1. Register — NFT is minted to this contract
        agentId = registry.register(agentURI);

        // 2. Transfer NFT to the user
        registry.safeTransferFrom(address(this), msg.sender, agentId);

        // 3. Forward exact price to treasury, refund excess
        (bool ok, ) = treasury.call{value: price}("");
        if (!ok) revert TransferFailed();

        uint256 excess = msg.value - price;
        if (excess > 0) {
            (bool refundOk, ) = msg.sender.call{value: excess}("");
            if (!refundOk) revert TransferFailed();
        }

        emit Minted(msg.sender, agentId, handleHash, price);
    }

    // ── Admin ──────────────────────────────────────────────────────────

    function setTrustedSigner(address newSigner) external onlyOwner {
        if (newSigner == address(0)) revert ZeroAddress();
        emit TrustedSignerUpdated(trustedSigner, newSigner);
        trustedSigner = newSigner;
    }

    function setTreasury(address newTreasury) external onlyOwner {
        if (newTreasury == address(0)) revert ZeroAddress();
        emit TreasuryUpdated(treasury, newTreasury);
        treasury = newTreasury;
    }

    function setPaused(bool paused_) external onlyOwner {
        paused = paused_;
        emit Paused(paused_);
    }

    /**
     * @notice Check if a handle has already been minted.
     * @param handleHash keccak256(abi.encodePacked(lowercaseHandle))
     */
    function isHandleMinted(bytes32 handleHash) external view returns (bool) {
        return mintedHandles[handleHash];
    }

    // ── Emergency ──────────────────────────────────────────────────────

    function emergencyWithdraw() external onlyOwner {
        (bool ok, ) = treasury.call{value: address(this).balance}("");
        if (!ok) revert TransferFailed();
    }

    function emergencyWithdrawToken(address token) external onlyOwner {
        if (token == address(0)) revert ZeroAddress();
        uint256 balance = IERC20(token).balanceOf(address(this));
        if (balance > 0) {
            bool ok = IERC20(token).transfer(treasury, balance);
            if (!ok) revert TransferFailed();
        }
    }

    function emergencyWithdrawNFT(address nft, uint256 tokenId) external onlyOwner {
        if (nft == address(0)) revert ZeroAddress();
        IERC721(nft).safeTransferFrom(address(this), treasury, tokenId);
    }

    // ── IERC721Receiver ────────────────────────────────────────────────

    function onERC721Received(
        address, address, uint256, bytes calldata
    ) external pure override returns (bytes4) {
        return IERC721Receiver.onERC721Received.selector;
    }
}
