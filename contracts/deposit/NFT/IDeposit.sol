// SPDX-License-Identifier: MIT
pragma solidity ^0.8.17;

interface IDeposit {
    event DepositEvent(bytes pubkey, uint256 weiAmount, bytes signature);

    event WithdrawnEvent(uint256 weiAmount);

    function deposit(
        bytes calldata pubkey,
        bytes calldata signature,
        uint256 tokenID
    ) external;

    function withdraw() external;

    function depositsOf(address account) external view returns (uint256);

    function depositUnlockingTimestamp(
        address payee
    ) external view returns (uint256);
}