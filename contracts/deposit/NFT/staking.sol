// SPDX-License-Identifier: MIT
pragma solidity ^0.8.17;

import "@openzeppelin/contracts/access/Ownable.sol";
import "@openzeppelin/contracts/token/ERC1155/ERC1155.sol";
import "@openzeppelin/contracts/token/ERC1155/IERC1155Receiver.sol";
import "@openzeppelin/contracts/token/ERC1155/IERC1155.sol";
import "@openzeppelin/contracts-upgradeable/proxy/utils/Initializable.sol";
import "./IDeposit.sol";
import "hardhat/console.sol";

contract NFT is ERC1155, Ownable {
    constructor() ERC1155("") Ownable() {}

    function mint(address to, uint256 id) external onlyOwner {
        //console.log(id);
        _mint(to, id, 1, "");
    }
}

contract Staking is IDeposit, IERC1155Receiver, Initializable {
    uint256 constant T50 = uint256(keccak256("50AMT"));
    uint256 constant T100 = uint256(keccak256("100AMT"));
    uint256 constant T500 = uint256(keccak256("500AMT"));

    IERC1155 token;
    mapping(address => uint256) depoistNFT;
    mapping(address => uint256) depositExpire;

    constructor() {}

    function supportsInterface(
        bytes4 interfaceId
    ) external pure returns (bool) {
        return
        interfaceId == type(IDeposit).interfaceId ||
        interfaceId == type(IERC1155Receiver).interfaceId;
    }

    function initialize(address addr) public initializer {
        token = IERC1155(addr);
    }

    function deposit(
        bytes calldata pubkey,
        bytes calldata signature,
        uint256 tokenID
    ) external {
        require(pubkey.length == 48, "Staking: invalid public key");
        require(signature.length == 96, "Staking: invalid signature");
        require(
            tokenID == T50 || tokenID == T100 || tokenID == T500,
            "Staking: not allowed token"
        );
        require(depoistNFT[msg.sender] == 0, "Staking: have deposited");
        require(
            token.balanceOf(msg.sender, tokenID) > 0,
            "Staking: Insuficient Allowance"
        );
        require(
            token.isApprovedForAll(msg.sender, address(this)),
            "Staking: caller is not approved"
        );

        token.safeTransferFrom(msg.sender, address(this), tokenID, 1, "");

        depoistNFT[msg.sender] = tokenID;
        depositExpire[msg.sender] = block.timestamp + 365 days;

        emit DepositEvent(pubkey, nftToAmount(tokenID), signature);
    }

    function withdraw() external {
        require(depoistNFT[msg.sender] > 0, "Staking: not deposited");
        require(
            depositExpire[msg.sender] <= block.timestamp,
            "Staking: token is locked"
        );

        uint256 id = depoistNFT[msg.sender];
        delete depoistNFT[msg.sender];
        delete depositExpire[msg.sender];

        token.safeTransferFrom(address(this), msg.sender, id, 1, "");
        emit WithdrawnEvent(nftToAmount(id));
    }

    function depositsOf(address account) external view returns (uint256) {
        return depoistNFT[account];
    }

    function depositUnlockingTimestamp(
        address account
    ) external view returns (uint256) {
        return depositExpire[account];
    }

    function disperseAMT(
        address payable[] calldata recipients,
        uint256[] calldata amounts
    ) external payable {
        require(
            recipients.length > 0 && recipients.length == amounts.length,
            "the lenght of receipts not equal amounts or length is 0 "
        );

        uint256 totalAmount;
        for (uint i = 0; i < amounts.length; ) {
            require(amounts[i] <= 0.7083 ether, "beyond the max rewards");
            unchecked {
                totalAmount += amounts[i];
                ++i;
            }
        }
        require(msg.value >= totalAmount, "Insufficient AMT sent");
        for (uint i = 0; i < recipients.length; i++) {
            recipients[i].transfer(amounts[i]);
        }
    }

    function nftToAmount(uint256 id) internal pure returns (uint256) {
        if (id == T50) {
            return 50 ether;
        }

        if (id == T100) {
            return 100 ether;
        }

        if (id == T500) {
            return 500 ether;
        }

        return 0;
    }

    function onERC1155Received(
        address /*operator*/,
        address /*from*/,
        uint256 /*id*/,
        uint256 /*value*/,
        bytes calldata /*data*/
    ) external pure returns (bytes4) {
        return
        bytes4(
            keccak256(
                "onERC1155Received(address,address,uint256,uint256,bytes)"
            )
        );
    }

    function onERC1155BatchReceived(
        address /*operator*/,
        address /*from*/,
        uint256[] calldata /*ids*/,
        uint256[] calldata /*values*/,
        bytes calldata /*data*/
    ) external returns (bytes4) {}
}