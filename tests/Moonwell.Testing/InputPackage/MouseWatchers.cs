using System;
using System.Collections.Generic;

namespace Moonwell.Testing
{
    /// <summary>A game component that logs every mouse reaction as "label moved", "label pressed Right" and "label released Left", into a log a case shares.</summary>
    public sealed class LogMouseGame : GameComponent, IMouseWatch
    {
        /// <summary>The log the case shares.</summary>
        private readonly List<string> log;

        /// <summary>The name in each line.</summary>
        private readonly string label;

        /// <summary>The name of a button, spelt out so that .NET and Lua write the same line.</summary>
        public static string Name(MouseButton button)
        {
            return button == MouseButton.Left ? "Left" : button == MouseButton.Middle ? "Middle" : "Right";
        }

        /// <summary>Makes a component that logs under a label.</summary>
        public LogMouseGame(List<string> log, string label)
        {
            this.log = log;
            this.label = label;
        }

        /// <summary>Logs the move.</summary>
        public void OnMouseMoved(Mouse mouse)
        {
            log.Add(label + " moved");
        }

        /// <summary>Logs the press.</summary>
        public void OnMousePressed(Mouse mouse, MouseButton button)
        {
            log.Add(label + " pressed " + LogMouseGame.Name(button));
        }

        /// <summary>Logs the release.</summary>
        public void OnMouseReleased(Mouse mouse, MouseButton button)
        {
            log.Add(label + " released " + LogMouseGame.Name(button));
        }
    }

    /// <summary>A player component that logs every mouse reaction as "label moved", "label pressed Right" and "label released Left", into a log a case shares.</summary>
    public sealed class LogMousePlayer : PlayerComponent, IMouseWatch
    {
        /// <summary>The log the case shares.</summary>
        private readonly List<string> log;

        /// <summary>The name in each line.</summary>
        private readonly string label;

        /// <summary>Makes a component that logs under a label.</summary>
        public LogMousePlayer(List<string> log, string label)
        {
            this.log = log;
            this.label = label;
        }

        /// <summary>Logs the move.</summary>
        public void OnMouseMoved(Mouse mouse)
        {
            log.Add(label + " moved");
        }

        /// <summary>Logs the press.</summary>
        public void OnMousePressed(Mouse mouse, MouseButton button)
        {
            log.Add(label + " pressed " + LogMouseGame.Name(button));
        }

        /// <summary>Logs the release.</summary>
        public void OnMouseReleased(Mouse mouse, MouseButton button)
        {
            log.Add(label + " released " + LogMouseGame.Name(button));
        }
    }

    /// <summary>A game component that writes only OnMousePressed.</summary>
    public sealed class PressOnlyMouse : GameComponent, IMouseWatch
    {
        /// <summary>The log the case shares.</summary>
        private readonly List<string> log;

        /// <summary>Makes a component that logs into a case's log.</summary>
        public PressOnlyMouse(List<string> log)
        {
            this.log = log;
        }

        /// <summary>Logs the press.</summary>
        public void OnMousePressed(Mouse mouse, MouseButton button)
        {
            log.Add("press only pressed " + LogMouseGame.Name(button));
        }
    }

    /// <summary>A game component whose OnMouseMoved throws.</summary>
    public sealed class ThrowingMouse : GameComponent, IMouseWatch
    {
        /// <summary>Throws.</summary>
        public void OnMouseMoved(Mouse mouse)
        {
            throw new InvalidOperationException("boom");
        }
    }
}
