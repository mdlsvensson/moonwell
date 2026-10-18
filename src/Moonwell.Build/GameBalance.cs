using War3Net.Build.Info;

namespace Moonwell.Build
{
    /// <summary>Maps the map's <c>Moonwell.Balance</c> to the tables the game reads and to the info file's game data option.</summary>
    public static class GameBalance
    {
        // The values of the runtime's Balance enum, read across the load context as numbers.
        private const int Default = 0;
        private const int MeleeV0 = 1;
        private const int CustomV0 = 2;
        private const int CustomV1 = 3;

        /// <summary>The name of the balance set whose tables the game reads for a balance.</summary>
        /// <param name="balance">The value of <c>Moonwell.Balance</c>.</param>
        /// <returns>Null for the base tables, else <c>melee_v0</c>, <c>custom_v0</c> or <c>custom_v1</c>.</returns>
        /// <exception cref="BuildException">The value is not one of the balances.</exception>
        public static string? SetOf(int balance)
        {
            switch (balance)
            {
                case Default:
                    return null;
                case MeleeV0:
                    return "melee_v0";
                case CustomV0:
                    return "custom_v0";
                case CustomV1:
                    return "custom_v1";
                default:
                    throw new BuildException($"Balance {balance} is not a balance of this version of Moonwell");
            }
        }

        /// <summary>Sets the info file's game data set and version to the option that makes the game read a balance's tables.</summary>
        /// <param name="info">The map's info file.</param>
        /// <param name="balance">The value of <c>Moonwell.Balance</c>.</param>
        /// <exception cref="BuildException">The value is not one of the balances.</exception>
        public static void Apply(MapInfo info, int balance)
        {
            switch (SetOf(balance))
            {
                case null:
                    info.GameDataSet = GameDataSet.Default;
                    info.GameDataVersion = GameDataVersion.Unset;
                    break;
                case "custom_v0":
                    info.GameDataSet = GameDataSet.Custom;
                    info.GameDataVersion = GameDataVersion.RoC;
                    break;
                case "custom_v1":
                    info.GameDataSet = GameDataSet.Custom;
                    info.GameDataVersion = GameDataVersion.TFT;
                    break;
                default:
                    info.GameDataSet = GameDataSet.Melee;
                    info.GameDataVersion = GameDataVersion.RoC;
                    break;
            }
        }
    }
}
