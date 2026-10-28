namespace Moonwell.MapFiles.Tests;

public class RawcodesTests
{
    [Fact]
    public void An_id_goes_to_its_number_and_back()
    {
        var id = Rawcodes.IdOf("hfoo");

        Assert.Equal(0x6F6F6668, id);
        Assert.Equal("hfoo", Rawcodes.TextOf(id));
        Assert.Equal("", Rawcodes.TextOf(0));
        Assert.Equal(0, Rawcodes.IdOf(""));
    }

    [Fact]
    public void From_refuses_an_id_that_is_not_four_bytes()
    {
        Assert.Equal(Rawcodes.IdOf("hfoo"), Rawcodes.From("hfoo", "id"));
        Assert.Throws<ArgumentException>(() => Rawcodes.From("hfo", "id"));
        Assert.Throws<ArgumentException>(() => Rawcodes.From("hfooo", "id"));
        Assert.Throws<ArgumentException>(() => Rawcodes.From("hfoĀ", "id"));
        Assert.Throws<ArgumentException>(() => Rawcodes.From(null, "id"));
    }
}
