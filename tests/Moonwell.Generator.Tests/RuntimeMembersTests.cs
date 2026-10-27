using Moonwell.GameData;
using Moonwell.Generator.Objects;

namespace Moonwell.Generator.Tests;

public class RuntimeMembersTests
{
    private static RuntimeMembers Read()
    {
        return RuntimeMembers.Read(Path.Combine(AppContext.BaseDirectory, "Moonwell.dll"));
    }

    [Theory]
    [InlineData(Kind.Unit, "Handle")]
    [InlineData(Kind.Unit, "Player")]
    [InlineData(Kind.Unit, "TypeId")]
    [InlineData(Kind.Unit, "Components")]
    [InlineData(Kind.Unit, "Of")]
    [InlineData(Kind.Unit, "OnDeath")]
    [InlineData(Kind.Item, "Handle")]
    [InlineData(Kind.Destructable, "Handle")]
    [InlineData(Kind.Ability, "Id")]
    [InlineData(Kind.Ability, "Level")]
    [InlineData(Kind.Ability, "Unit")]
    [InlineData(Kind.Buff, "Duration")]
    [InlineData(Kind.Buff, "Tick")]
    [InlineData(Kind.Buff, "Stacking")]
    [InlineData(Kind.Buff, "Source")]
    [InlineData(Kind.Buff, "Actor")]
    [InlineData(Kind.Doodad, "ToString")]
    [InlineData(Kind.Upgrade, "Equals")]
    [InlineData(Kind.Upgrade, "MemberwiseClone")]
    public void names_of_the_runtime_class_and_its_bases(Kind kind, string name)
    {
        Assert.Contains(name, Read().Of(kind));
    }

    [Theory]
    [InlineData(Kind.Unit, "get_Handle")]
    [InlineData(Kind.Unit, ".ctor")]
    [InlineData(Kind.Unit, "Mask")]
    [InlineData(Kind.Unit, "owner")]
    [InlineData(Kind.Unit, "Duration")]
    [InlineData(Kind.Ability, "Handle")]
    [InlineData(Kind.Buff, "Remove")]
    public void a_name_the_class_does_not_inherit_is_free(Kind kind, string name)
    {
        Assert.DoesNotContain(name, Read().Of(kind));
    }

    [Fact]
    public void a_dll_that_is_not_built_is_an_error()
    {
        var error = Assert.Throws<FileNotFoundException>(() => RuntimeMembers.Read(Path.Combine(AppContext.BaseDirectory, "nowhere", "Moonwell.dll")));

        Assert.Contains("is not built; run dotnet build on src/Moonwell first", error.Message);
    }
}
