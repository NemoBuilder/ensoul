// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/token/ERC721/IERC721Receiver.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";
import "@openzeppelin/contracts/utils/cryptography/ECDSA.sol";
import "@openzeppelin/contracts/utils/cryptography/MessageHashUtils.sol";

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
