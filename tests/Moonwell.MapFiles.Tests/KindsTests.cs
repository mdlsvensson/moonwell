using Moonwell.GameData;

namespace Moonwell.MapFiles.Tests;

public class KindsTests
{
    [Fact]
    public void Every_kind_has_its_files()
    {
        var expected = new[]
        {
            (Kind.Unit, "w3u", "unit"),
            (Kind.Item, "w3t", "item"),
            (Kind.Destructable, "w3b", "destructable"),
            (Kind.Doodad, "w3d", "doodad"),
            (Kind.Ability, "w3a", "ability"),
            (Kind.Buff, "w3h", "buff"),
            (Kind.Upgrade, "w3q", "upgrade"),
        };

        Assert.Equal(expected.Select(one => one.Item1), Kinds.All);
        foreach (var (kind, extension, key) in expected)
        {
            Assert.Equal(extension, Kinds.Extension(kind));
            Assert.Equal(key, Kinds.Key(kind));
            Assert.Equal("war3map." + extension, Kinds.MainName(kind));
            Assert.Equal("war3mapSkin." + extension, Kinds.SkinName(kind));
        }
    }

    [Fact]
    public void A_key_names_its_kind()
    {
        Assert.Equal(Kind.Ability, Kinds.FromKey("ability"));
        Assert.Null(Kinds.FromKey("hero"));
        foreach (var kind in Kinds.All)
        {
            Assert.Equal(kind, Kinds.FromKey(Kinds.Key(kind)));
        }
    }

    [Fact]
    public void Only_doodads_abilities_and_upgrades_have_levels()
    {
        var withLevels = Kinds.All.Where(Kinds.HasLevels).ToList();

        Assert.Equal(new[] { Kind.Doodad, Kind.Ability, Kind.Upgrade }, withLevels);
    }
}
