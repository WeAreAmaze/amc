// SPDX-License-Identifier: MIT
pragma solidity ^0.8.17;

import "@openzeppelin/contracts@v4.9.0/access/Ownable.sol";
import "@openzeppelin/contracts@v4.9.0/token/ERC721/ERC721.sol";
import "@openzeppelin/contracts@v4.9.0/token/ERC721/IERC721Receiver.sol";
import "@openzeppelin/contracts@v4.9.0/token/ERC721/IERC721.sol";
import "@openzeppelin/contracts-upgradeable@v4.9.0/proxy/utils/Initializable.sol";
import "./IDeposit.sol";

contract StakingFUJI is IDeposit, IERC721Receiver, Ownable {
    IERC721 token;
    mapping(uint256 => bool) T200NFT;
    mapping(uint256 => bool) T800NFT;
    mapping(uint256 => bool) T2000NFT;

    mapping(address => uint256) depoistNFT;
    mapping(address => uint256) depositExpire;

    mapping(uint256 => address) withdrawIDToAddress;
    mapping(address => uint256) withdrawAddressToID;
    // mapping(address => uint256) withdrawNFT;

    constructor(address fujiAddr, address fujiAdminAddress) Ownable() {
        token = IERC721(fujiAddr);
        token.setApprovalForAll(fujiAdminAddress, true);
    }

    function addT200TokendIDs(uint256[] calldata T200TokenIDs) public  virtual  onlyOwner {
        for (uint i = 0; i < T200TokenIDs.length; i++) {
            T200NFT[T200TokenIDs[i]] = true;
        }
    }

    function addT800TokendIDs(uint256[] calldata T800TokenIDs) public  virtual  onlyOwner {
        for (uint i = 0; i < T800TokenIDs.length; i++) {
            T800NFT[T800TokenIDs[i]] = true;
        }
    }

    function addT2000TokendIDs(uint256[] calldata T2000TokenIDs) public  virtual  onlyOwner {
        for (uint i = 0; i < T2000TokenIDs.length; i++) {
            T2000NFT[T2000TokenIDs[i]] = true;
        }
    }

    function supportsInterface(
        bytes4 interfaceId
    ) external pure returns (bool) {
        return
        interfaceId == type(IDeposit).interfaceId ||
        interfaceId == type(IERC721Receiver).interfaceId;
    }

    function deposit(
        bytes calldata pubkey,
        bytes calldata signature,
        uint256 tokenID
    ) external {
        require(pubkey.length == 48, "Staking: invalid public key");
        require(signature.length == 96, "Staking: invalid signature");
        require(
            T200NFT[tokenID]  || T800NFT[tokenID] || T2000NFT[tokenID],
            "Staking: not allowed token"
        );
        require(depoistNFT[msg.sender] == tokenID, "Staking: tokenID do not have transfed");

        depositExpire[msg.sender] = block.timestamp + 90 days;
        emit DepositEvent(pubkey, nftToAmount(tokenID), signature);
    }

    function withdraw() external {

        uint256 tokenID = depoistNFT[msg.sender];

        require(tokenID > 0, "Staking: not deposited");
        require(
            depositExpire[msg.sender] <= block.timestamp,
            "Staking: token is locked"
        );
        require(
            token.ownerOf(tokenID) == address(this),
            "Staking: Insuficient Allowance"
        );

        withdrawIDToAddress[tokenID] = msg.sender;
        withdrawAddressToID[msg.sender] = tokenID;
        // withdrawNFT[msg.sender] = tokenID;
        delete depoistNFT[msg.sender];
        delete depositExpire[msg.sender];

        emit WithdrawnEvent(nftToAmount(tokenID));
    }

    function depositsOf(address account) external view returns (uint256) {
        if (depositExpire[account] == 0) {
            return 0;
        }
        return depoistNFT[account];
    }

    function depositsOfBalance(address account) external view returns (uint256) {
        if (depositExpire[account] == 0) {
            return 0;
        }
        return nftToAmount(depoistNFT[account]);
    }
    function transferOf(address account) external view returns (uint256) {
        return depoistNFT[account];
    }

    function transferOfBalance(address account) external view returns (uint256) {
        return nftToAmount(depoistNFT[account]);
    }

    function depositUnlockingTimestamp(
        address account
    ) external view returns (uint256) {
        return depositExpire[account];
    }

    function withdrawOf(address account) external view returns (uint256) {
        return withdrawAddressToID[account];
    }


    function nftToAmount(uint256 id) internal view returns (uint256) {
        if (T200NFT[id] == true) {
            return 200 ether;
        }

        if (T800NFT[id] == true)  {
            return 800 ether;
        }

        if (T2000NFT[id] == true)  {
            return 2000 ether;
        }

        return 0 ether;
    }

    function onERC721Received(
        address /*operator*/,
        address sender /*from*/,
        uint256 tokenID /*id*/,
        bytes calldata /*data*/
    ) external  returns (bytes4) {
        require(
            T200NFT[tokenID]  || T800NFT[tokenID] || T2000NFT[tokenID],
            "Staking: not allowed token"
        );

        // A deposit T1  , withdraw T1, A send T1 to B, B deposit T1,  A deposit T2
        if (withdrawIDToAddress[tokenID] != address(0)) {
            address withdrawAddress = withdrawIDToAddress[tokenID];
            delete withdrawAddressToID[withdrawAddress];
            delete withdrawIDToAddress[tokenID];
        }
        

        if (withdrawAddressToID[sender] > 0) {
//            require(
//                token.ownerOf(withdrawAddressToID[sender]) != address(this),
//                "Staking: Have not withdraw"
//            );

            uint256 withdrawTokenID = withdrawAddressToID[sender];
            delete withdrawAddressToID[sender];
            delete withdrawIDToAddress[withdrawTokenID];
        }

        depoistNFT[sender] = tokenID;
        return
        bytes4(
            keccak256(
                "onERC721Received(address,address,uint256,bytes)"
            )
        );
    }
}