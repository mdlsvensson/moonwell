using Moonwell;

namespace TwoReplaces
{
    public sealed class TwoReplacesMap : Map
    {
    }

    [Moonwell.Objects.Replaces]
    public class A : Moonwell.Objects.Units.Human.Footman
    {
    }

    [Moonwell.Objects.Replaces]
    public class B : Moonwell.Objects.Units.Human.Footman
    {
    }
}
