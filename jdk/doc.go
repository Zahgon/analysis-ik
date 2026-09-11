// Package jdk reproduces the observable behaviour of the Java platform APIs the
// IK Analyzer was written against.
//
// The analyzer is specified in terms of Java chars: every offset it reports,
// every dictionary lookup it performs and every buffer boundary it respects is
// counted in UTF-16 code units. Go has no such type, so the behaviour the JDK
// used to supply for free — UTF-16 text, java.io.Reader's chunked reads,
// Character.UnicodeBlock classification, String.trim and the XML dialect
// java.util.Properties reads — is reproduced here rather than approximated with
// runes, bytes or an off-the-shelf parser.
package jdk
