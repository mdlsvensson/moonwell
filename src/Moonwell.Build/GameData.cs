using Moonwell.GameData;
using War3Net.Build.Info;

namespace Moonwell.Build
{
    /// <summary>
    /// The map's game data option, <c>Moonwell.DataSet</c> and <c>Moonwell.DataVersion</c> together: what the info file
    /// says and which tables the game reads for the pair (measured in the game; see docs/game-facts.md).
    /// </summary>
    public sealed class GameData
    {
        // The values of the runtime's DataSet and DataVersion enums, read across the load context as numbers.
        private const int Default = 0;
        private const int Custom = 1;
        private const int Melee = 2;
        private const int ReignOfChaos = 0;
        private const int FrozenThrone = 1;
        private const int ForsakenKingdom = 2;

        private GameData(GameDataSet infoSet, GameDataVersion infoVersion, string? valueSet, string? existenceSet)
        {
            InfoSet = infoSet;
            InfoVersion = infoVersion;
            ValueSet = valueSet;
            ExistenceSet = existenceSet;
        }

        /// <summary>The info file's game data set.</summary>
        public GameDataSet InfoSet { get; }

        /// <summary>The info file's game data version.</summary>
        public GameDataVersion InfoVersion { get; }

        /// <summary>The tables the game takes values from: null for the base tables, else <c>custom_v0</c>, <c>custom_v1</c> or <c>melee_v0</c>.</summary>
        public string? ValueSet { get; }

        /// <summary>The tables that decide whether a standard object exists: the value set, except under Melee / Frozen Throne.</summary>
        public string? ExistenceSet { get; }

        /// <summary>The pair for the numbers of <c>Moonwell.DataSet</c> and <c>Moonwell.DataVersion</c>.</summary>
        /// <param name="dataSet">The value of <c>Moonwell.DataSet</c>.</param>
        /// <param name="dataVersion">The value of <c>Moonwell.DataVersion</c>.</param>
        /// <returns>The game data option.</returns>
        /// <exception cref="BuildException">A value is not one of the enum's values.</exception>
        public static GameData Of(int dataSet, int dataVersion)
        {
            var (set, setOption) = dataSet switch
            {
                Default => (GameDataSet.Default, DataSetOption.Default),
                Custom => (GameDataSet.Custom, DataSetOption.Custom),
                Melee => (GameDataSet.Melee, DataSetOption.Melee),
                _ => throw new BuildException($"DataSet {dataSet} is not a data set of this version of Moonwell"),
            };
            var (version, versionOption) = dataVersion switch
            {
                ReignOfChaos => (GameDataVersion.RoC, DataVersionOption.ReignOfChaos),
                FrozenThrone => (GameDataVersion.TFT, DataVersionOption.FrozenThrone),
                ForsakenKingdom => (GameDataVersion.ForsakenKingdom, DataVersionOption.ForsakenKingdom),
                _ => throw new BuildException($"DataVersion {dataVersion} is not a data version of this version of Moonwell"),
            };

            var tables = DataOptions.TablesOf(setOption, versionOption);
            return new GameData(set, version, NullForBase(tables.Values), NullForBase(tables.Objects));
        }

        private static string? NullForBase(string tables)
        {
            return tables == "base" ? null : tables;
        }

        /// <summary>Words the pair as World Editor does.</summary>
        /// <param name="set">The info file's game data set.</param>
        /// <param name="version">The info file's game data version.</param>
        /// <returns>For example <c>Melee / Frozen Throne</c>.</returns>
        public static string Describe(GameDataSet set, GameDataVersion version)
        {
            var versionWords = version switch
            {
                GameDataVersion.RoC => "Reign of Chaos",
                GameDataVersion.TFT => "Frozen Throne",
                _ => "Forsaken Kingdom",
            };
            return $"{set} / {versionWords}";
        }

        /// <summary>Sets the info file's game data set and version to this option.</summary>
        /// <param name="info">The map's info file.</param>
        public void Apply(MapInfo info)
        {
            info.GameDataSet = InfoSet;
            info.GameDataVersion = InfoVersion;
        }

        /// <summary>Words the pair as World Editor does.</summary>
        /// <returns>For example <c>Melee / Frozen Throne</c>.</returns>
        public override string ToString()
        {
            return Describe(InfoSet, InfoVersion);
        }
    }
}
