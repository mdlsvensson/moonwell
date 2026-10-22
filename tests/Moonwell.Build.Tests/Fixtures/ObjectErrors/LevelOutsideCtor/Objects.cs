#pragma warning disable MW0015 // the analyzer reports this too; the fixture is for the build's own check
using Moonwell;

namespace LevelOutsideCtor
{
    public sealed class LevelOutsideCtorMap : Map
    {
    }

    public class Veteran : Moonwell.Objects.Human.Units.Footman
    {
        public Veteran()
        {
            Level = 3;

            // A lambda inside a constructor counts as the constructor.
            System.Action raise = () => Level = 4;
            raise();
        }

        public void Promote()
        {
            Level = 5;
        }
    }
}
