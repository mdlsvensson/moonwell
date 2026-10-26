using System.IO;
using System.Linq;
using System.Xml.Linq;

namespace Moonwell.Tests;

public class DamageDocsTests
{
    // The damage interfaces' methods carry @CSharpLua.Ignore in their doc comments; a map author's editor shows only the
    // summary beside it, so each method needs one.
    [Fact]
    public void EveryDamageReactionHasAHoverSummary()
    {
        var xml = Path.ChangeExtension(typeof(Hit).Assembly.Location, ".xml");
        var members = XDocument.Load(xml).Descendants("member")
            .Where(member => ((string)member.Attribute("name")!).StartsWith("M:Moonwell.IDamage"))
            .ToList();
        Assert.Equal(14, members.Count);
        var bare = members.Where(member => member.Element("summary") == null).Select(member => (string)member.Attribute("name")!).ToList();
        Assert.True(bare.Count == 0, "reactions without a summary: " + string.Join(", ", bare));
    }
}
