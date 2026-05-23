// SPDX-License-Identifier: MIT
pragma solidity ^0.8.20;

/// @title EnsoulGalaxyNFT — V4 Galaxy ownership token
/// @notice Minimal ERC-721 implementation (no OpenZeppelin import to stay
///         consistent with the rest of the contracts/ folder which uses
///         flattened standalone style). Only the platform minter can mint;
///         transfers follow ERC-721 standard rules so secondary markets work.
///
/// One Galaxy = one NFT. tokenId is the off-chain Galaxy index (incrementing).
/// The off-chain Galaxy.id (UUID) is stored as bytes32 metadata so explorers
/// can cross-reference.
contract EnsoulGalaxyNFT {
    // ─── ERC-721 storage ─────────────────────────────────────────────────

    string public constant name = "Ensoul Galaxy";
    string public constant symbol = "GALAXY";

    mapping(uint256 => address) private _owners;
    mapping(address => uint256) private _balances;
    mapping(uint256 => address) private _tokenApprovals;
    mapping(address => mapping(address => bool)) private _operatorApprovals;

    // ─── Galaxy metadata ─────────────────────────────────────────────────

    /// @notice tokenId → off-chain Galaxy UUID (encoded big-endian as bytes32).
    mapping(uint256 => bytes32) public galaxyId;
    /// @notice tokenId → metadata URI (IPFS / centralised JSON).
    mapping(uint256 => string) private _tokenURI;
    /// @notice tokenId → founder address at mint time (for revenue share).
    mapping(uint256 => address) public founderOf;

    // ─── Roles ───────────────────────────────────────────────────────────

    address public owner;
    address public minter;
    uint256 public nextTokenId = 1;

    // ─── Events ──────────────────────────────────────────────────────────

    event Transfer(address indexed from, address indexed to, uint256 indexed tokenId);
    event Approval(address indexed owner, address indexed approved, uint256 indexed tokenId);
    event ApprovalForAll(address indexed owner, address indexed operator, bool approved);

    event GalaxyMinted(
        uint256 indexed tokenId,
        bytes32 indexed galaxyId,
        address indexed founder,
        string uri
    );
    event MinterRotated(address indexed oldMinter, address indexed newMinter);
    event OwnerTransferred(address indexed oldOwner, address indexed newOwner);

    modifier onlyOwner() {
        require(msg.sender == owner, "not owner");
        _;
    }

    modifier onlyMinter() {
        require(msg.sender == minter, "not minter");
        _;
    }

    constructor(address _minter) {
        owner = msg.sender;
        minter = _minter;
        emit OwnerTransferred(address(0), msg.sender);
        emit MinterRotated(address(0), _minter);
    }

    // ─── Mint ────────────────────────────────────────────────────────────

    /// @notice Mint one Galaxy NFT to `to`. Called by the platform minter
    ///         immediately after a GalaxyApplication is approved.
    /// @return tokenId The freshly minted token id.
    function mintGalaxy(
        address to,
        bytes32 gid,
        string calldata uri
    ) external onlyMinter returns (uint256 tokenId) {
        require(to != address(0), "zero to");
        tokenId = nextTokenId++;
        _mint(to, tokenId);
        galaxyId[tokenId] = gid;
        _tokenURI[tokenId] = uri;
        founderOf[tokenId] = to;
        emit GalaxyMinted(tokenId, gid, to, uri);
    }

    /// @notice Update tokenURI (e.g. when off-chain metadata changes).
    function setTokenURI(uint256 tokenId, string calldata uri) external onlyOwner {
        require(_owners[tokenId] != address(0), "no token");
        _tokenURI[tokenId] = uri;
    }

    function tokenURI(uint256 tokenId) external view returns (string memory) {
        require(_owners[tokenId] != address(0), "no token");
        return _tokenURI[tokenId];
    }

    // ─── ERC-721 core ────────────────────────────────────────────────────

    function balanceOf(address who) external view returns (uint256) {
        require(who != address(0), "zero addr");
        return _balances[who];
    }

    function ownerOf(uint256 tokenId) public view returns (address) {
        address o = _owners[tokenId];
        require(o != address(0), "no token");
        return o;
    }

    function approve(address to, uint256 tokenId) external {
        address o = ownerOf(tokenId);
        require(msg.sender == o || _operatorApprovals[o][msg.sender], "not authorised");
        _tokenApprovals[tokenId] = to;
        emit Approval(o, to, tokenId);
    }

    function getApproved(uint256 tokenId) external view returns (address) {
        require(_owners[tokenId] != address(0), "no token");
        return _tokenApprovals[tokenId];
    }

    function setApprovalForAll(address operator, bool approved) external {
        require(operator != msg.sender, "self approve");
        _operatorApprovals[msg.sender][operator] = approved;
        emit ApprovalForAll(msg.sender, operator, approved);
    }

    function isApprovedForAll(address o, address operator) external view returns (bool) {
        return _operatorApprovals[o][operator];
    }

    function transferFrom(address from, address to, uint256 tokenId) public {
        _transfer(from, to, tokenId);
    }

    function safeTransferFrom(address from, address to, uint256 tokenId) external {
        _transfer(from, to, tokenId);
        // EOA-only safe — V4.0 does not need ERC-721 receiver callback because
        // launches all go through EOA or known multisig.
    }

    function safeTransferFrom(address from, address to, uint256 tokenId, bytes calldata) external {
        _transfer(from, to, tokenId);
    }

    // ─── Internal ────────────────────────────────────────────────────────

    function _mint(address to, uint256 tokenId) internal {
        require(_owners[tokenId] == address(0), "exists");
        _owners[tokenId] = to;
        _balances[to] += 1;
        emit Transfer(address(0), to, tokenId);
    }

    function _transfer(address from, address to, uint256 tokenId) internal {
        require(_owners[tokenId] == from, "not owner");
        require(to != address(0), "zero to");
        address spender = msg.sender;
        require(
            spender == from ||
                _tokenApprovals[tokenId] == spender ||
                _operatorApprovals[from][spender],
            "not authorised"
        );
        delete _tokenApprovals[tokenId];
        _balances[from] -= 1;
        _balances[to] += 1;
        _owners[tokenId] = to;
        emit Transfer(from, to, tokenId);
    }

    // ─── Admin ───────────────────────────────────────────────────────────

    function setMinter(address newMinter) external onlyOwner {
        require(newMinter != address(0), "zero minter");
        emit MinterRotated(minter, newMinter);
        minter = newMinter;
    }

    function transferOwnership(address newOwner) external onlyOwner {
        require(newOwner != address(0), "zero owner");
        emit OwnerTransferred(owner, newOwner);
        owner = newOwner;
    }

    // ─── ERC-165 ─────────────────────────────────────────────────────────

    function supportsInterface(bytes4 interfaceId) external pure returns (bool) {
        return
            interfaceId == 0x01ffc9a7 || // ERC-165
            interfaceId == 0x80ac58cd || // ERC-721
            interfaceId == 0x5b5e139f;   // ERC-721 Metadata
    }
}
