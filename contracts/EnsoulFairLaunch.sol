// SPDX-License-Identifier: MIT
// EnsoulFairLaunch — per-Galaxy fair launch state machine.
//
// Lifecycle for one galaxyId:
//   Open      — platform calls openLaunch(...) once per galaxy
//   Deposit   — anyone can deposit() BNB during [start, end)
//   Token wired — platform deploys EnsoulCommunityToken off-chain (minting
//                full supply to THIS contract) and calls setToken(gid, addr)
//   Finalize  — after end, anyone calls finalize(gid):
//                 - if totalRaised >= minRaise AND token is set → success;
//                   founder+platform BNB split forwarded; depositors claim()
//                 - else → refundable; depositors call refund()
//   Claim / Refund — terminal user actions
//
// Why off-chain token deploy?
//   Deploying a new contract inside finalize() balloons bytecode and creates
//   awkward audit surface; off-chain deploy + setToken(gid, addr) keeps this
//   contract small and lets the token bytecode be inspected independently.
//   The launchpad refuses finalize success unless token.balanceOf(this) >= supply.
//
// galaxyId encoding matches EnsoulEpochRegistry / EnsoulGalaxyNFT (UUID
// left-aligned into bytes32).
pragma solidity ^0.8.20;

interface IERC20Minimal {
    function balanceOf(address) external view returns (uint256);
    function transfer(address to, uint256 value) external returns (bool);
}

contract EnsoulFairLaunch {
    address public owner;
    address public platform; // receives platform fee BPS of raised BNB
    uint16  public platformFeeBps; // e.g. 500 = 5%

    struct Launch {
        address founder;        // receives (1 - platformFeeBps) of raised BNB
        uint64  start;
        uint64  end;
        uint128 minRaise;
        uint128 maxRaise;
        uint128 totalRaised;
        uint256 supply;         // tokens to distribute to depositors
        address token;          // EnsoulCommunityToken — set via setToken
        bool    finalized;
        bool    succeeded;      // valid only after finalized
    }

    mapping(bytes32 => Launch) public launches;
    mapping(bytes32 => mapping(address => uint256)) public deposits;
    mapping(bytes32 => mapping(address => bool)) public claimed;

    event LaunchOpened(bytes32 indexed gid, address indexed founder, uint64 start, uint64 end, uint128 minRaise, uint128 maxRaise, uint256 supply);
    event TokenWired(bytes32 indexed gid, address indexed token);
    event Deposited(bytes32 indexed gid, address indexed who, uint256 amount, uint128 totalRaised);
    event Finalized(bytes32 indexed gid, bool succeeded, uint128 totalRaised, uint256 founderShare, uint256 platformShare);
    event Claimed(bytes32 indexed gid, address indexed who, uint256 amount);
    event Refunded(bytes32 indexed gid, address indexed who, uint256 amount);
    event OwnerTransferred(address indexed prev, address indexed next);
    event PlatformUpdated(address platform, uint16 feeBps);

    modifier onlyOwner() { require(msg.sender == owner, "FL: not owner"); _; }

    constructor(address _platform, uint16 _platformFeeBps) {
        require(_platform != address(0), "FL: platform zero");
        require(_platformFeeBps <= 1000, "FL: fee too high"); // hard cap 10%
        owner = msg.sender;
        platform = _platform;
        platformFeeBps = _platformFeeBps;
    }

    // ─── admin ───────────────────────────────────────────────────────────

    function transferOwnership(address next) external onlyOwner {
        require(next != address(0), "FL: next zero");
        emit OwnerTransferred(owner, next);
        owner = next;
    }

    function setPlatform(address _platform, uint16 _feeBps) external onlyOwner {
        require(_platform != address(0), "FL: platform zero");
        require(_feeBps <= 1000, "FL: fee too high");
        platform = _platform;
        platformFeeBps = _feeBps;
        emit PlatformUpdated(_platform, _feeBps);
    }

    function openLaunch(
        bytes32 gid,
        address founder,
        uint64  start,
        uint64  end,
        uint128 minRaise,
        uint128 maxRaise,
        uint256 supply
    ) external onlyOwner {
        require(launches[gid].founder == address(0), "FL: exists");
        require(founder != address(0), "FL: founder zero");
        require(end > start && start >= block.timestamp, "FL: bad window");
        require(maxRaise == 0 || maxRaise >= minRaise, "FL: bad raise bounds");
        require(supply > 0, "FL: supply zero");
        launches[gid] = Launch({
            founder: founder,
            start: start,
            end: end,
            minRaise: minRaise,
            maxRaise: maxRaise,
            totalRaised: 0,
            supply: supply,
            token: address(0),
            finalized: false,
            succeeded: false
        });
        emit LaunchOpened(gid, founder, start, end, minRaise, maxRaise, supply);
    }

    function setToken(bytes32 gid, address token) external onlyOwner {
        Launch storage L = launches[gid];
        require(L.founder != address(0), "FL: no launch");
        require(!L.finalized, "FL: finalized");
        require(L.token == address(0), "FL: token set");
        require(token != address(0), "FL: token zero");
        L.token = token;
        emit TokenWired(gid, token);
    }

    // ─── user flow ───────────────────────────────────────────────────────

    function deposit(bytes32 gid) external payable {
        Launch storage L = launches[gid];
        require(L.founder != address(0), "FL: no launch");
        require(!L.finalized, "FL: finalized");
        require(block.timestamp >= L.start && block.timestamp < L.end, "FL: window");
        require(msg.value > 0, "FL: zero");
        uint128 newTotal = L.totalRaised + uint128(msg.value);
        if (L.maxRaise > 0) {
            require(newTotal <= L.maxRaise, "FL: cap");
        }
        L.totalRaised = newTotal;
        deposits[gid][msg.sender] += msg.value;
        emit Deposited(gid, msg.sender, msg.value, newTotal);
    }

    function finalize(bytes32 gid) external {
        Launch storage L = launches[gid];
        require(L.founder != address(0), "FL: no launch");
        require(!L.finalized, "FL: done");
        require(block.timestamp >= L.end, "FL: window");

        L.finalized = true;
        bool ok = L.totalRaised >= L.minRaise && L.token != address(0)
            && IERC20Minimal(L.token).balanceOf(address(this)) >= L.supply;
        L.succeeded = ok;

        uint256 founderShare;
        uint256 platformShare;
        if (ok) {
            uint256 raised = uint256(L.totalRaised);
            platformShare = (raised * platformFeeBps) / 10_000;
            founderShare = raised - platformShare;
            if (platformShare > 0) {
                (bool s1, ) = platform.call{value: platformShare}("");
                require(s1, "FL: pay platform");
            }
            if (founderShare > 0) {
                (bool s2, ) = L.founder.call{value: founderShare}("");
                require(s2, "FL: pay founder");
            }
        }
        emit Finalized(gid, ok, L.totalRaised, founderShare, platformShare);
    }

    function claim(bytes32 gid) external {
        Launch storage L = launches[gid];
        require(L.finalized && L.succeeded, "FL: not claimable");
        require(!claimed[gid][msg.sender], "FL: claimed");
        uint256 dep = deposits[gid][msg.sender];
        require(dep > 0, "FL: nothing");
        claimed[gid][msg.sender] = true;
        // share = dep * supply / totalRaised  (mulDiv, no overflow at 2^128 * 2^256)
        uint256 amount = (dep * L.supply) / uint256(L.totalRaised);
        require(IERC20Minimal(L.token).transfer(msg.sender, amount), "FL: token xfer");
        emit Claimed(gid, msg.sender, amount);
    }

    function refund(bytes32 gid) external {
        Launch storage L = launches[gid];
        require(L.finalized && !L.succeeded, "FL: not refundable");
        uint256 dep = deposits[gid][msg.sender];
        require(dep > 0, "FL: nothing");
        deposits[gid][msg.sender] = 0;
        (bool s, ) = msg.sender.call{value: dep}("");
        require(s, "FL: refund");
        emit Refunded(gid, msg.sender, dep);
    }

    // Reject random BNB sends — protect against losing funds outside the deposit flow.
    receive() external payable { revert("FL: use deposit"); }
}
