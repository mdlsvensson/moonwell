using Moonwell.GameData;

namespace Moonwell.Generator.Tests;

public class DataOptionsTests
{
    [Theory]
    [InlineData(DataSetOption.Default, DataVersionOption.ReignOfChaos, "custom_v0", "custom_v0")]
    [InlineData(DataSetOption.Custom, DataVersionOption.ReignOfChaos, "custom_v0", "custom_v0")]
    [InlineData(DataSetOption.Melee, DataVersionOption.ReignOfChaos, "melee_v0", "melee_v0")]
    [InlineData(DataSetOption.Default, DataVersionOption.FrozenThrone, "custom_v1", "custom_v1")]
    [InlineData(DataSetOption.Custom, DataVersionOption.FrozenThrone, "custom_v1", "custom_v1")]
    [InlineData(DataSetOption.Melee, DataVersionOption.FrozenThrone, "base", "custom_v1")]
    [InlineData(DataSetOption.Default, DataVersionOption.ForsakenKingdom, "base", "base")]
    [InlineData(DataSetOption.Custom, DataVersionOption.ForsakenKingdom, "base", "base")]
    [InlineData(DataSetOption.Melee, DataVersionOption.ForsakenKingdom, "base", "base")]
    public void The_tables_follow_the_option(DataSetOption set, DataVersionOption version, string values, string objects)
    {
        Assert.Equal(new DataTables(values, objects), DataOptions.TablesOf(set, version));
    }
}
