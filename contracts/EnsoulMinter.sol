// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/token/ERC721/IERC721Receiver.sol";
import "@openzeppelin/contracts/utils/ReentrancyGuard.sol";

/**
 * @title EnsoulMinter
 * @notice Wrapper around ERC-8004 IdentityRegistry that charges a mint fee in BNB.
 *
 * Flow:
 *   1. User calls mint(agentURI) with msg.value >= mintFee
 *   2. Contract calls registry.register(agentURI) — NFT is minted to this contract
 *   3. Contract transfers the NFT to the user via safeTransferFrom
 *   4. BNB fee is forwarded to the treasury
 *
 * The contract implements IERC721Receiver so it can receive the NFT from _safeMint.
 */

/// @dev Minimal interface for the ERC-8004 IdentityRegistry register + transfer functions.
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

contract EnsoulMinter is Ownable, IERC721Receiver, ReentrancyGuard {
    // ── State ──────────────────────────────────────────────────────────
    IIdentityRegistry public immutable registry;
    address public treasury;
    uint256 public mintFee;  // in wei (e.g. ~1 USD worth of BNB)
    bool public paused;

    // ── Events ─────────────────────────────────────────────────────────
    event Minted(address indexed user, uint256 indexed agentId, uint256 fee);
    event MintFeeUpdated(uint256 oldFee, uint256 newFee);
    event TreasuryUpdated(address oldTreasury, address newTreasury);
    event Paused(bool isPaused);

    // ── Errors ─────────────────────────────────────────────────────────
    error InsufficientFee(uint256 required, uint256 provided);
    error MintingPaused();
    error TransferFailed();
    error ZeroAddress();

    // ── Constructor ────────────────────────────────────────────────────
    constructor(
        address registry_,
        address treasury_,
        uint256 mintFee_
    ) Ownable(msg.sender) {
        if (registry_ == address(0) || treasury_ == address(0)) revert ZeroAddress();
        registry = IIdentityRegistry(registry_);
        treasury = treasury_;
        mintFee = mintFee_;
    }

    // ── Core ───────────────────────────────────────────────────────────

    /**
     * @notice Mint an ERC-8004 identity NFT. Must send at least `mintFee` BNB.
     * @param agentURI The agent registration file URI (data URI or IPFS/HTTPS).
     * @return agentId The newly minted agent's token ID.
     */
    function mint(string calldata agentURI) external payable nonReentrant returns (uint256 agentId) {
        if (paused) revert MintingPaused();
        if (msg.value < mintFee) revert InsufficientFee(mintFee, msg.value);

        // 1. Register — NFT is minted to this contract (msg.sender = address(this))
        agentId = registry.register(agentURI);

        // 2. Transfer NFT to the actual user
        registry.safeTransferFrom(address(this), msg.sender, agentId);

        // 3. Forward BNB to treasury
        (bool ok, ) = treasury.call{value: msg.value}("");
        if (!ok) revert TransferFailed();

        emit Minted(msg.sender, agentId, msg.value);
    }

    // ── Admin ──────────────────────────────────────────────────────────

    function setMintFee(uint256 newFee) external onlyOwner {
        emit MintFeeUpdated(mintFee, newFee);
        mintFee = newFee;
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
     * @notice Emergency withdraw any stuck BNB (should not happen in normal flow).
     */
    function emergencyWithdraw() external onlyOwner {
        (bool ok, ) = treasury.call{value: address(this).balance}("");
        if (!ok) revert TransferFailed();
    }

    /**
     * @notice Rescue ERC-20 tokens accidentally sent to this contract.
     * @param token The ERC-20 token contract address.
     */
    function emergencyWithdrawToken(address token) external onlyOwner {
        if (token == address(0)) revert ZeroAddress();
        uint256 balance = IERC20(token).balanceOf(address(this));
        if (balance > 0) {
            bool ok = IERC20(token).transfer(treasury, balance);
            if (!ok) revert TransferFailed();
        }
    }

    /**
     * @notice Rescue ERC-721 NFTs accidentally sent to this contract.
     * @param nft The ERC-721 contract address.
     * @param tokenId The token ID to rescue.
     */
    function emergencyWithdrawNFT(address nft, uint256 tokenId) external onlyOwner {
        if (nft == address(0)) revert ZeroAddress();
        IERC721(nft).safeTransferFrom(address(this), treasury, tokenId);
    }

    // ── IERC721Receiver ────────────────────────────────────────────────

    /**
     * @notice Required to receive ERC-721 tokens via _safeMint.
     */
    function onERC721Received(
        address, /* operator */
        address, /* from */
        uint256, /* tokenId */
        bytes calldata /* data */
    ) external pure override returns (bytes4) {
        return IERC721Receiver.onERC721Received.selector;
    }
}
