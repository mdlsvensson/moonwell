#pragma warning disable MW0012 // the analyzer reports this too; the fixture is for the build's own check
using Moonwell;

namespace HeroLevel
{
    public sealed class HeroLevelMap : Map
    {
    }

    public class Veteran : Moonwell.Objects.Human.Units.Paladin
    {
        public Veteran()
        {
            Set("ulev", 3);
        }
    }
}
